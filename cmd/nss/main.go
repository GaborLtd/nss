package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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
var errNonRetryableSSH = errors.New("non-retryable SSH error")
var errTerminated = errors.New("nss terminated")

const (
	remoteAttachCommand       = `PATH="$HOME/.local/bin:$PATH" exec nssd attach`
	nonTTYDiagnostic          = "stdin is not a terminal; use --no-tty for non-interactive use"
	nonRetryableSSHDiagnostic = "請檢查 SSH 的 ProxyCommand、RemoteCommand 或 wrapper 是否又啟動了 nss"
)

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
	if len(os.Args) > 1 && os.Args[1] == "--ssh-askpass" {
		if err := runSSHAskpass(); err != nil {
			os.Exit(1)
		}
		return
	}
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
	fmt.Printf("已更新 nss 與 nssd %s：%s\n", result.Tag, strings.Join(result.Paths, ", "))
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
	statusANSI := !config.noTTY && term.IsTerminal(int(os.Stderr.Fd()))
	statusActive := false
	defer func() {
		if statusActive {
			_ = clearReconnectStatus(os.Stderr, statusANSI)
		}
	}()
	for {
		if statusActive {
			_ = clearReconnectStatus(os.Stderr, statusANSI)
			statusActive = false
		}
		err := runConnection(config, inputCh, inputClosed, resizeCh, interrupts)
		if err == nil {
			return nil
		}
		if errors.Is(err, errTerminated) {
			return nil
		}
		if errors.Is(err, errRemoteNSSDNotFound) || errors.Is(err, errNonRetryableSSH) {
			return err
		}
		if first {
			_ = writeReconnectStatus(os.Stderr, "connection lost; reconnecting", statusANSI)
			statusActive = true
			first = false
		}
		_ = writeReconnectStatus(os.Stderr, fmt.Sprintf("retrying in %s: %v", backoff, err), statusANSI)
		statusActive = true
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

	sshArgs := []string{"-T"}
	if configPath := userSSHConfigPath(); configPath != "" {
		sshArgs = append(sshArgs, "-F", configPath)
	}
	sshArgs = append(sshArgs, config.host, remoteAttachCommand)
	cmd := exec.Command("ssh", sshArgs...)
	var askpass *askpassBridge
	var err error
	if !config.noTTY && term.IsTerminal(int(os.Stdin.Fd())) {
		askpass, err = newAskpassBridge()
		if err != nil {
			return err
		}
		cmd.Env = askpass.childEnvironment()
		cmd.ExtraFiles = askpass.childFiles()
		defer askpass.close()
	}
	var sshStderr strings.Builder
	if config.noTTY || !term.IsTerminal(int(os.Stderr.Fd())) {
		cmd.Stderr = io.MultiWriter(os.Stderr, &sshStderr)
	} else {
		// 互動式 terminal 由 nss 統一顯示 reconnect 狀態，避免 ssh 錯誤訊息破壞畫面。
		cmd.Stderr = &sshStderr
	}
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
		if askpass != nil {
			askpass.closeChildFiles()
		}
		return err
	}
	if askpass != nil {
		askpass.closeChildFiles()
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
	var askpassRequests <-chan string
	if askpass != nil {
		askpassRequests = askpass.requests
	}

	for {
		select {
		case prompt, ok := <-askpassRequests:
			if !ok {
				askpassRequests = nil
				continue
			}
			passphrase, err := readPassphrase(inputCh, inputClosed, interrupts, os.Stderr, prompt)
			if err != nil {
				return err
			}
			if err := askpass.respond(passphrase); err != nil {
				return errDisconnected
			}
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

func userSSHConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".ssh", "config")
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return ""
	}
	return path
}

func readPassphrase(inputCh <-chan []byte, inputClosed <-chan struct{}, interrupts <-chan os.Signal, promptWriter io.Writer, prompt string) (string, error) {
	if _, err := io.WriteString(promptWriter, prompt); err != nil {
		return "", err
	}
	var passphrase []byte
	for {
		select {
		case data := <-inputCh:
			for _, character := range data {
				switch character {
				case '\r', '\n':
					_, _ = io.WriteString(promptWriter, "\r\n")
					return string(passphrase), nil
				case 0x03:
					_, _ = io.WriteString(promptWriter, "^C\r\n")
					return "", errTerminated
				case 0x08, 0x7f:
					if len(passphrase) > 0 {
						passphrase = passphrase[:len(passphrase)-1]
					}
				default:
					if character >= 0x20 {
						passphrase = append(passphrase, character)
					}
				}
			}
		case <-inputClosed:
			return "", errTerminated
		case <-interrupts:
			return "", errTerminated
		}
	}
}

func classifySSHExit(waitErr error, stderr string) error {
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) && exitErr.ExitCode() == 127 {
		detail := compactSSHError(stderr)
		if detail != "" {
			return fmt.Errorf("%w: %s; 請先在遠端安裝 nssd，並啟動 `nssd serve`", errRemoteNSSDNotFound, detail)
		}
		return fmt.Errorf("%w: 請先在遠端安裝 nssd，並啟動 `nssd serve`", errRemoteNSSDNotFound)
	}
	return classifySSHDiagnostic(stderr)
}

func classifySSHDisconnect(waitCh <-chan error, stderr string) error {
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case waitErr := <-waitCh:
		return classifySSHExit(waitErr, stderr)
	case <-timer.C:
		return classifySSHDiagnostic(stderr)
	}
}

func classifySSHDiagnostic(stderr string) error {
	detail := compactSSHError(stderr)
	if detail == "" {
		return errDisconnected
	}
	if strings.Contains(detail, nonTTYDiagnostic) {
		return fmt.Errorf("%w: %s; %s", errNonRetryableSSH, detail, nonRetryableSSHDiagnostic)
	}
	return fmt.Errorf("%w: %s", errDisconnected, detail)
}

func compactSSHError(stderr string) string {
	detail := strings.Join(strings.Fields(stderr), " ")
	if len(detail) > 240 {
		return detail[:240] + "..."
	}
	return detail
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

func writeReconnectStatus(w io.Writer, message string, ansi bool) error {
	if !ansi {
		_, err := fmt.Fprintf(w, "[nss] %s\n", message)
		return err
	}
	// 儲存 remote cursor，在上方一行顯示狀態，再還原 cursor。
	// 不往下換行，避免 prompt 在 terminal 底部時觸發 scroll，造成後續全螢幕程式畫面錯位。
	message = fitReconnectStatus(message, reconnectStatusColumns())
	_, err := fmt.Fprintf(w, "\x1b7\x1b[1A\r\x1b[2K[nss] %s\x1b8", message)
	return err
}

func clearReconnectStatus(w io.Writer, ansi bool) error {
	if !ansi {
		return nil
	}
	// 狀態列位於 remote cursor 的上一行；清除後還原原本的 cursor 位置。
	_, err := io.WriteString(w, "\x1b7\x1b[1A\r\x1b[2K\x1b8")
	return err
}

func reconnectStatusColumns() int {
	if _, cols, err := term.GetSize(int(os.Stderr.Fd())); err == nil && cols > 0 {
		return cols
	}
	return 80
}

func fitReconnectStatus(message string, columns int) string {
	const prefixWidth = len("[nss] ")
	if columns <= prefixWidth {
		return ""
	}
	maxRunes := columns - prefixWidth
	runes := []rune(message)
	if len(runes) <= maxRunes {
		return message
	}
	return string(runes[:maxRunes])
}
