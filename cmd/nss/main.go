package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gaborltd/nss/internal/protocol"
	"github.com/gaborltd/nss/internal/session"
	updater "github.com/gaborltd/nss/internal/update"
	"github.com/gaborltd/nss/internal/version"
	"golang.org/x/term"
)

var errDisconnected = errors.New("ssh transport disconnected")
var errRemoteNSSDNotFound = errors.New("remote nssd command not found")
var errTerminated = errors.New("nss terminated")

const remoteAttachCommand = `PATH="$HOME/.local/bin:$PATH" exec nssd attach`

type clientConfig struct {
	host           string
	sessionID      string
	secret         string
	noTTY          bool
	initialBackoff time.Duration
	maxBackoff     time.Duration
}

type frameEvent struct {
	frame protocol.Frame
	err   error
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(version.String("nss"))
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "update" {
		if err := runUpdate(os.Args[2:], "nss"); err != nil {
			fmt.Fprintln(os.Stderr, "nss:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "nss:", err)
		os.Exit(1)
	}
}

func runUpdate(args []string, binaryName string) error {
	flags := flag.NewFlagSet(binaryName+" update", flag.ContinueOnError)
	repository := os.Getenv("NSS_REPOSITORY")
	if repository == "" {
		repository = "gaborltd/nss"
	}
	repositoryFlag := flags.String("repository", repository, "GitHub repository")
	versionFlag := flags.String("version", "latest", "release version")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("usage: %s update [--version vX.Y.Z]", binaryName)
	}
	result, err := updater.Run(updater.Config{
		Repository: *repositoryFlag,
		Version:    *versionFlag,
	})
	if err != nil {
		return err
	}
	fmt.Printf("已更新 %s %s 到 %s\n", binaryName, result.Tag, result.Path)
	return nil
}

func run(args []string) error {
	flags := flag.NewFlagSet("nss", flag.ContinueOnError)
	sessionID := flags.String("session-id", "", "existing session ID")
	secret := flags.String("session-secret", "", "existing session secret")
	noTTY := flags.Bool("no-tty", false, "do not configure the local terminal as raw TTY")
	initialBackoff := flags.Duration("reconnect-initial", time.Second, "initial reconnect delay")
	maxBackoff := flags.Duration("reconnect-max", 30*time.Second, "maximum reconnect delay")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: nss [flags] <ssh-host>")
	}
	if *initialBackoff <= 0 || *maxBackoff < *initialBackoff {
		return errors.New("reconnect delays are invalid")
	}

	id := *sessionID
	if id == "" {
		var err error
		id, err = session.RandomToken(16)
		if err != nil {
			return fmt.Errorf("generate session id: %w", err)
		}
	}
	secretValue := *secret
	if secretValue == "" {
		var err error
		secretValue, err = session.RandomToken(32)
		if err != nil {
			return fmt.Errorf("generate session secret: %w", err)
		}
	}

	config := clientConfig{
		host:           flags.Arg(0),
		sessionID:      id,
		secret:         secretValue,
		noTTY:          *noTTY,
		initialBackoff: *initialBackoff,
		maxBackoff:     *maxBackoff,
	}

	var oldState *term.State
	if !config.noTTY {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return errors.New("stdin is not a terminal; use --no-tty for non-interactive use")
		}
		var err error
		oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("configure local terminal: %w", err)
		}
		defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()
	}
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(interrupts)

	inputCh := make(chan []byte, 1)
	inputClosed := make(chan struct{})
	go readInput(inputCh, inputClosed)

	resizeCh := make(chan [2]uint16, 1)
	resizeStop := make(chan struct{})
	if !config.noTTY {
		go watchResize(resizeCh, resizeStop)
		defer close(resizeStop)
	}

	backoff := config.initialBackoff
	first := true
	for {
		err := runConnection(config, inputCh, inputClosed, resizeCh, interrupts)
		if err == nil {
			return nil
		}
		if errors.Is(err, errTerminated) {
			return nil
		}
		if errors.Is(err, errRemoteNSSDNotFound) {
			return err
		}
		if first {
			fmt.Fprintln(os.Stderr, "[nss] connection lost; reconnecting")
			first = false
		}
		fmt.Fprintf(os.Stderr, "[nss] retrying in %s: %v\n", backoff, err)
		timer := time.NewTimer(backoff)
		select {
		case <-timer.C:
		case <-interrupts:
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		case <-inputClosed:
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		}
		if backoff < config.maxBackoff {
			backoff *= 2
			if backoff > config.maxBackoff {
				backoff = config.maxBackoff
			}
		}
	}
}

func runConnection(config clientConfig, inputCh <-chan []byte, inputClosed <-chan struct{}, resizeCh <-chan [2]uint16, interrupts <-chan os.Signal) error {
	// 丟棄斷線期間最多暫存的一筆輸入，避免重新連線後盲送舊指令。
	drainInput(inputCh)

	cmd := exec.Command("ssh", "-T", config.host, remoteAttachCommand)
	var sshStderr strings.Builder
	cmd.Stderr = io.MultiWriter(os.Stderr, &sshStderr)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return err
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-waitCh:
		case <-time.After(time.Second):
		}
	}()

	rows, cols := terminalSize()
	open, err := protocol.EncodeOpen(protocol.OpenRequest{
		SessionID: config.sessionID,
		Secret:    config.secret,
		Rows:      rows,
		Cols:      cols,
	})
	if err != nil {
		return err
	}
	if err := protocol.Write(stdin, open); err != nil {
		return errDisconnected
	}

	frames := make(chan frameEvent, 4)
	go readFrames(stdout, frames)
	opened := false

	for {
		select {
		case event := <-frames:
			if event.err != nil {
				return classifySSHDisconnect(waitCh, sshStderr.String())
			}
			switch event.frame.Type {
			case protocol.TypeOpenOK:
				if _, err := protocol.DecodeOpenResponse(event.frame); err != nil {
					return err
				}
				opened = true
			case protocol.TypeOpenError:
				return fmt.Errorf("remote attach rejected: %s", strings.TrimSpace(string(event.frame.Payload)))
			case protocol.TypeData:
				if !opened {
					return errors.New("received PTY data before attach acknowledgement")
				}
				if _, err := os.Stdout.Write(event.frame.Payload); err != nil {
					return err
				}
			case protocol.TypeClose:
				return nil
			default:
				return fmt.Errorf("unexpected frame type: %d", event.frame.Type)
			}
		case data := <-inputCh:
			if !opened {
				continue
			}
			if err := protocol.Write(stdin, protocol.Frame{Type: protocol.TypeData, Payload: data}); err != nil {
				return errDisconnected
			}
		case size := <-resizeCh:
			payload := []byte{byte(size[0] >> 8), byte(size[0]), byte(size[1] >> 8), byte(size[1])}
			if opened && protocol.Write(stdin, protocol.Frame{Type: protocol.TypeResize, Payload: payload}) != nil {
				return errDisconnected
			}
		case <-inputClosed:
			if opened {
				_ = protocol.Write(stdin, protocol.Frame{Type: protocol.TypeClose})
			}
			return nil
		case waitErr := <-waitCh:
			return classifySSHExit(waitErr, sshStderr.String())
		case <-interrupts:
			return errTerminated
		}
	}
}

func classifySSHExit(waitErr error, stderr string) error {
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) && exitErr.ExitCode() == 127 {
		detail := strings.TrimSpace(stderr)
		if detail != "" {
			return fmt.Errorf("%w: %s; 請先在遠端安裝 nssd，並啟動 `nssd serve`", errRemoteNSSDNotFound, detail)
		}
		return fmt.Errorf("%w: 請先在遠端安裝 nssd，並啟動 `nssd serve`", errRemoteNSSDNotFound)
	}
	return errDisconnected
}

func classifySSHDisconnect(waitCh <-chan error, stderr string) error {
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case waitErr := <-waitCh:
		return classifySSHExit(waitErr, stderr)
	case <-timer.C:
		return errDisconnected
	}
}

func readInput(inputCh chan<- []byte, inputClosed chan<- struct{}) {
	buf := make([]byte, 32*1024)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			data := append([]byte(nil), buf[:n]...)
			select {
			case inputCh <- data:
			default:
				// 斷線期間不排隊，避免重新連線後執行過時輸入。
			}
		}
		if err != nil {
			close(inputClosed)
			return
		}
	}
}

func readFrames(reader io.Reader, frames chan<- frameEvent) {
	for {
		frame, err := protocol.Read(reader)
		frames <- frameEvent{frame: frame, err: err}
		if err != nil {
			return
		}
	}
}

func watchResize(resizeCh chan<- [2]uint16, stop <-chan struct{}) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)
	defer signal.Stop(signals)
	for {
		select {
		case <-signals:
			rows, cols := terminalSize()
			select {
			case resizeCh <- [2]uint16{rows, cols}:
			default:
			}
		case <-stop:
			return
		}
	}
}

func terminalSize() (uint16, uint16) {
	rows, cols := uint16(24), uint16(80)
	if r, c, err := term.GetSize(int(os.Stdin.Fd())); err == nil && r > 0 && c > 0 {
		rows, cols = uint16(r), uint16(c)
	}
	return rows, cols
}

func drainInput(inputCh <-chan []byte) {
	for {
		select {
		case <-inputCh:
		default:
			return
		}
	}
}
