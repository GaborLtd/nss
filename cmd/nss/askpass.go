package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
)

const (
	askpassRequestFDEnv  = "NSS_ASKPASS_REQUEST_FD"
	askpassResponseFDEnv = "NSS_ASKPASS_RESPONSE_FD"
	askpassRequestFD     = 3
	askpassResponseFD    = 4
	maxAskpassFrameSize  = 64 * 1024
)

// askpassBridge 讓 SSH 的 passphrase prompt 回到 nss，由目前 terminal 處理輸入。
type askpassBridge struct {
	requestReader       *os.File
	responseWriter      *os.File
	childRequestWriter  *os.File
	childResponseReader *os.File
	requests            chan string
}

func newAskpassBridge() (*askpassBridge, error) {
	requestReader, childRequestWriter, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("create SSH askpass request pipe: %w", err)
	}
	childResponseReader, responseWriter, err := os.Pipe()
	if err != nil {
		_ = requestReader.Close()
		_ = childRequestWriter.Close()
		return nil, fmt.Errorf("create SSH askpass response pipe: %w", err)
	}

	bridge := &askpassBridge{
		requestReader:       requestReader,
		responseWriter:      responseWriter,
		childRequestWriter:  childRequestWriter,
		childResponseReader: childResponseReader,
		requests:            make(chan string, 1),
	}
	go bridge.readRequests()
	return bridge, nil
}

func (bridge *askpassBridge) childFiles() []*os.File {
	return []*os.File{bridge.childRequestWriter, bridge.childResponseReader}
}

func (bridge *askpassBridge) childEnvironment() []string {
	env := os.Environ()
	env = replaceEnvironmentValue(env, "SSH_ASKPASS", mustExecutablePath())
	env = replaceEnvironmentValue(env, "SSH_ASKPASS_REQUIRE", "force")
	// OpenSSH 仍要求 DISPLAY 存在， 即使 SSH_ASKPASS_REQUIRE=force。
	env = replaceEnvironmentValue(env, "DISPLAY", ":0")
	env = replaceEnvironmentValue(env, askpassRequestFDEnv, strconv.Itoa(askpassRequestFD))
	env = replaceEnvironmentValue(env, askpassResponseFDEnv, strconv.Itoa(askpassResponseFD))
	return env
}

func (bridge *askpassBridge) respond(passphrase string) error {
	return writeAskpassFrame(bridge.responseWriter, []byte(passphrase))
}

func (bridge *askpassBridge) close() {
	_ = bridge.requestReader.Close()
	_ = bridge.responseWriter.Close()
	_ = bridge.childRequestWriter.Close()
	_ = bridge.childResponseReader.Close()
}

func (bridge *askpassBridge) closeChildFiles() {
	_ = bridge.childRequestWriter.Close()
	_ = bridge.childResponseReader.Close()
}

func (bridge *askpassBridge) readRequests() {
	defer close(bridge.requests)
	for {
		payload, err := readAskpassFrame(bridge.requestReader)
		if err != nil {
			return
		}
		bridge.requests <- string(payload)
	}
}

func runSSHAskpass() error {
	request, err := askpassFileFromEnvironment(askpassRequestFDEnv)
	if err != nil {
		return err
	}
	defer request.Close()
	response, err := askpassFileFromEnvironment(askpassResponseFDEnv)
	if err != nil {
		return err
	}
	defer response.Close()

	if err := writeAskpassFrame(request, []byte(joinAskpassPrompt(os.Args[2:]))); err != nil {
		return err
	}
	passphrase, err := readAskpassFrame(response)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(passphrase)
	return err
}

func askpassFileFromEnvironment(name string) (*os.File, error) {
	value := os.Getenv(name)
	fd, err := strconv.Atoi(value)
	if err != nil || fd < 0 {
		return nil, fmt.Errorf("invalid %s", name)
	}
	return os.NewFile(uintptr(fd), name), nil
}

func joinAskpassPrompt(args []string) string {
	if len(args) == 0 {
		return "SSH authentication requires input: "
	}
	return args[0]
}

func writeAskpassFrame(writer io.Writer, payload []byte) error {
	if len(payload) > maxAskpassFrameSize {
		return fmt.Errorf("SSH askpass message is too large")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	_, err := writer.Write(payload)
	return err
}

func readAskpassFrame(reader io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size > maxAskpassFrameSize {
		return nil, fmt.Errorf("SSH askpass message is too large")
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func replaceEnvironmentValue(env []string, key, value string) []string {
	prefix := key + "="
	filtered := env[:0]
	for _, item := range env {
		if len(item) >= len(prefix) && item[:len(prefix)] == prefix {
			continue
		}
		filtered = append(filtered, item)
	}
	return append(filtered, prefix+value)
}

func mustExecutablePath() string {
	path, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	return path
}
