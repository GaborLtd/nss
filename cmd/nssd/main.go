package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gaborltd/nss/internal/protocol"
	"github.com/gaborltd/nss/internal/runtimepath"
	"github.com/gaborltd/nss/internal/session"
	updater "github.com/gaborltd/nss/internal/update"
	"github.com/gaborltd/nss/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		if err := runServe(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "attach":
		if err := runAttach(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "list":
		if err := runAdminList(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "close":
		if err := runAdminClose(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "update":
		if err := runUpdate(os.Args[2:], "nssd"); err != nil {
			fatal(err)
		}
	case "--version", "version":
		fmt.Println(version.String("nssd"))
	default:
		usage()
		os.Exit(2)
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

func runServe(args []string) error {
	flags := flag.NewFlagSet("nssd serve", flag.ContinueOnError)
	defaultSocket, err := runtimepath.SocketPath()
	if err != nil {
		return err
	}
	defaultState, err := runtimepath.StateDir()
	if err != nil {
		return err
	}
	socketPath := flags.String("socket", defaultSocket, "Unix socket path")
	stateDir := flags.String("state-dir", defaultState, "session state directory")
	shell := flags.String("shell", "", "default shell path")
	maxSpoolMB := flags.Int64("max-spool-mb", 4, "maximum output spool per session")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *maxSpoolMB < 0 {
		return errors.New("max-spool-mb cannot be negative")
	}

	if err := os.MkdirAll(filepath.Dir(*socketPath), 0700); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}
	if err := removeStaleSocket(*socketPath); err != nil {
		return err
	}
	listener, err := net.Listen("unix", *socketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", *socketPath, err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(*socketPath)
	}()
	if err := os.Chmod(*socketPath, 0600); err != nil {
		return fmt.Errorf("protect socket: %w", err)
	}

	manager, err := session.NewManager(session.Config{
		StateDir:      *stateDir,
		DefaultShell:  *shell,
		MaxSpoolBytes: *maxSpoolMB << 20,
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, formatReadyMessage(*socketPath, *stateDir, *maxSpoolMB))
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)
	go func() {
		<-stop
		_ = listener.Close()
		manager.CloseAll()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept attach connection: %w", err)
		}
		go handleAttach(conn, manager)
	}
}

func formatReadyMessage(socketPath, stateDir string, maxSpoolMB int64) string {
	return fmt.Sprintf("nssd: ready; socket=%s; state-dir=%s; max-spool=%d MiB", socketPath, stateDir, maxSpoolMB)
}

func handleAttach(conn net.Conn, manager *session.Manager) {
	defer conn.Close()
	first, err := protocol.Read(conn)
	if err != nil {
		return
	}
	switch first.Type {
	case protocol.TypeOpen:
		handleOpen(conn, manager, first)
	case protocol.TypeAdminList:
		handleAdminList(conn, manager)
	case protocol.TypeAdminClose:
		handleAdminClose(conn, manager, first)
	}
}

func handleOpen(conn net.Conn, manager *session.Manager, first protocol.Frame) {
	req, err := protocol.DecodeOpen(first)
	if err != nil {
		_ = protocol.Write(conn, protocol.Frame{Type: protocol.TypeOpenError, Payload: []byte(err.Error())})
		return
	}
	attachment, replay, err := manager.Attach(req)
	if err != nil {
		_ = protocol.Write(conn, protocol.Frame{Type: protocol.TypeOpenError, Payload: []byte(err.Error())})
		return
	}
	defer attachment.Detach()

	response, err := protocol.EncodeOpenResponse(protocol.OpenResponse{
		SessionID: req.SessionID,
		Replayed:  int64(len(replay)),
	})
	if err != nil || protocol.Write(conn, response) != nil {
		return
	}
	for len(replay) > 0 {
		n := len(replay)
		if n > protocol.MaxFrameSize {
			n = protocol.MaxFrameSize
		}
		if err := protocol.Write(conn, protocol.Frame{Type: protocol.TypeData, Payload: replay[:n]}); err != nil {
			return
		}
		replay = replay[n:]
	}

	go func() {
		for frame := range attachment.Frames() {
			if err := protocol.Write(conn, frame); err != nil {
				_ = conn.Close()
				return
			}
		}
	}()

	for {
		frame, err := protocol.Read(conn)
		if err != nil {
			return
		}
		switch frame.Type {
		case protocol.TypeData:
			if err := attachment.WriteInput(frame.Payload); err != nil {
				return
			}
		case protocol.TypeResize:
			if len(frame.Payload) != 4 {
				return
			}
			rows := uint16(frame.Payload[0])<<8 | uint16(frame.Payload[1])
			cols := uint16(frame.Payload[2])<<8 | uint16(frame.Payload[3])
			if err := attachment.Resize(rows, cols); err != nil {
				return
			}
		case protocol.TypeClose:
			attachment.CloseSession()
			return
		default:
			return
		}
	}
}

func handleAdminList(conn net.Conn, manager *session.Manager) {
	items := manager.List()
	protocolItems := make([]protocol.SessionInfo, 0, len(items))
	for _, item := range items {
		protocolItems = append(protocolItems, protocol.SessionInfo{
			SessionID: item.ID,
			Attached:  item.Attached,
		})
	}
	frame, err := protocol.EncodeSessionInfo(protocolItems)
	if err != nil {
		return
	}
	_ = protocol.Write(conn, frame)
}

func handleAdminClose(conn net.Conn, manager *session.Manager, first protocol.Frame) {
	req, err := protocol.DecodeAdminClose(first)
	if err != nil {
		_ = protocol.Write(conn, protocol.Frame{Type: protocol.TypeOpenError, Payload: []byte(err.Error())})
		return
	}
	if err := manager.Close(req.SessionID); err != nil {
		_ = protocol.Write(conn, protocol.Frame{Type: protocol.TypeOpenError, Payload: []byte(err.Error())})
		return
	}
	_ = protocol.Write(conn, protocol.Frame{Type: protocol.TypeAdminOK})
}

func runAttach(args []string) error {
	flags := flag.NewFlagSet("nssd attach", flag.ContinueOnError)
	defaultSocket, err := runtimepath.SocketPath()
	if err != nil {
		return err
	}
	socketPath := flags.String("socket", defaultSocket, "Unix socket path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	conn, err := net.Dial("unix", *socketPath)
	if err != nil {
		return fmt.Errorf("connect to nssd: %w", err)
	}
	defer conn.Close()

	errCh := make(chan error, 2)
	go func() {
		_, copyErr := io.Copy(conn, os.Stdin)
		if unixConn, ok := conn.(*net.UnixConn); ok {
			_ = unixConn.CloseWrite()
		}
		errCh <- copyErr
	}()
	go func() {
		_, copyErr := io.Copy(os.Stdout, conn)
		errCh <- copyErr
	}()
	return <-errCh
}

func runAdminList(args []string) error {
	conn, err := dialAdmin("nssd list", args)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := protocol.Write(conn, protocol.Frame{Type: protocol.TypeAdminList}); err != nil {
		return err
	}
	frame, err := protocol.Read(conn)
	if err != nil {
		return err
	}
	if frame.Type == protocol.TypeOpenError {
		return errors.New(string(frame.Payload))
	}
	items, err := protocol.DecodeSessionInfo(frame)
	if err != nil {
		return err
	}
	fmt.Println("SESSION_ID\tATTACHED")
	for _, item := range items {
		fmt.Printf("%s\t%t\n", item.SessionID, item.Attached)
	}
	return nil
}

func runAdminClose(args []string) error {
	flags := flag.NewFlagSet("nssd close", flag.ContinueOnError)
	defaultSocket, err := runtimepath.SocketPath()
	if err != nil {
		return err
	}
	socketPath := flags.String("socket", defaultSocket, "Unix socket path")
	sessionID := flags.String("session-id", "", "session ID to close")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *sessionID == "" {
		return errors.New("--session-id is required")
	}
	conn, err := net.Dial("unix", *socketPath)
	if err != nil {
		return fmt.Errorf("connect to nssd: %w", err)
	}
	defer conn.Close()
	request, err := protocol.EncodeAdminClose(protocol.AdminCloseRequest{SessionID: *sessionID})
	if err != nil {
		return err
	}
	if err := protocol.Write(conn, request); err != nil {
		return err
	}
	frame, err := protocol.Read(conn)
	if err != nil {
		return err
	}
	if frame.Type == protocol.TypeOpenError {
		return errors.New(string(frame.Payload))
	}
	if frame.Type != protocol.TypeAdminOK {
		return fmt.Errorf("unexpected admin response type: %d", frame.Type)
	}
	return nil
}

func dialAdmin(name string, args []string) (net.Conn, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	defaultSocket, err := runtimepath.SocketPath()
	if err != nil {
		return nil, err
	}
	socketPath := flags.String("socket", defaultSocket, "Unix socket path")
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	conn, err := net.Dial("unix", *socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to nssd: %w", err)
	}
	return conn, nil
}

func removeStaleSocket(path string) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to remove non-socket path: %s", path)
	}
	probe, err := net.DialTimeout("unix", path, 100*time.Millisecond)
	if err == nil {
		_ = probe.Close()
		return fmt.Errorf("nssd socket is already in use: %s", path)
	}
	return os.Remove(path)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: nssd serve|attach|list|close|update|--version")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "nssd:", err)
	os.Exit(1)
}
