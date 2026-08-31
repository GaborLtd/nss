package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoteAttachCommandAddsUserLocalBin(t *testing.T) {
	if !strings.Contains(remoteAttachCommand, `PATH="$HOME/.local/bin:$PATH"`) {
		t.Fatalf("remote attach command = %q, expected ~/.local/bin in PATH", remoteAttachCommand)
	}
	if !strings.HasSuffix(remoteAttachCommand, "exec nssd attach") {
		t.Fatalf("remote attach command = %q, expected nssd attach", remoteAttachCommand)
	}
}

func TestUserSSHConfigPathUsesCurrentUserConfig(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(sshDir, "config")
	if err := os.WriteFile(configPath, []byte("Host mdev3\n  HostName 192.168.19.102\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	if got := userSSHConfigPath(); got != configPath {
		t.Fatalf("userSSHConfigPath() = %q, want %q", got, configPath)
	}
}

func TestClassifySSHExitDetectsMissingRemoteNSSD(t *testing.T) {
	err := exec.Command("sh", "-c", "exit 127").Run()
	classified := classifySSHExit(err, "zsh: command not found: nssd")
	if !errors.Is(classified, errRemoteNSSDNotFound) {
		t.Fatalf("classified error = %v, expected remote nssd error", classified)
	}
	if !strings.Contains(classified.Error(), "nssd serve") {
		t.Fatalf("classified error = %v, expected setup guidance", classified)
	}
}

func TestClassifySSHExitTreatsTransportFailureAsReconnectable(t *testing.T) {
	err := exec.Command("sh", "-c", "exit 255").Run()
	if classified := classifySSHExit(err, ""); !errors.Is(classified, errDisconnected) {
		t.Fatalf("classified error = %v, expected disconnected error", classified)
	}
}

func TestClassifySSHExitCompactsTransportDiagnostic(t *testing.T) {
	err := exec.Command("sh", "-c", "exit 255").Run()
	classified := classifySSHExit(err, "Read from remote host 192.168.19.102: Connection reset by peer\nclient_loop: send disconnect: Broken pipe\n")
	if !errors.Is(classified, errDisconnected) {
		t.Fatalf("classified error = %v, expected disconnected error", classified)
	}
	if got := classified.Error(); got != "ssh transport disconnected: Read from remote host 192.168.19.102: Connection reset by peer client_loop: send disconnect: Broken pipe" {
		t.Fatalf("classified error = %q", got)
	}
}

func TestClassifySSHExitStopsRetryForNonTTYNSSDiagnostic(t *testing.T) {
	err := exec.Command("sh", "-c", "exit 1").Run()
	classified := classifySSHExit(err, "nss: "+nonTTYDiagnostic)
	if !errors.Is(classified, errNonRetryableSSH) {
		t.Fatalf("classified error = %v, expected non-retryable SSH error", classified)
	}
	if errors.Is(classified, errDisconnected) {
		t.Fatalf("classified error = %v, should not be retryable transport error", classified)
	}
	if !strings.Contains(classified.Error(), "ProxyCommand") {
		t.Fatalf("classified error = %v, expected SSH configuration guidance", classified)
	}
}

func TestReconnectStatusPreservesCursorWithANSI(t *testing.T) {
	var output bytes.Buffer
	if err := writeReconnectStatus(&output, "retrying in 1s: ssh transport disconnected", true); err != nil {
		t.Fatalf("writeReconnectStatus() error = %v", err)
	}
	want := "\x1b7\x1b[1A\r\x1b[2K[nss] retrying in 1s: ssh transport disconnected\x1b8"
	if output.String() != want {
		t.Fatalf("status output = %q, want %q", output.String(), want)
	}
}

func TestReconnectStatusUsesPlainLinesWithoutANSI(t *testing.T) {
	var output bytes.Buffer
	if err := writeReconnectStatus(&output, "connection lost; reconnecting", false); err != nil {
		t.Fatalf("writeReconnectStatus() error = %v", err)
	}
	if output.String() != "[nss] connection lost; reconnecting\n" {
		t.Fatalf("status output = %q", output.String())
	}
}

func TestClearReconnectStatusPreservesCursorWithANSI(t *testing.T) {
	var output bytes.Buffer
	if err := clearReconnectStatus(&output, true); err != nil {
		t.Fatalf("clearReconnectStatus() error = %v", err)
	}
	want := "\x1b7\x1b[1A\r\x1b[2K\x1b8"
	if output.String() != want {
		t.Fatalf("clear output = %q, want %q", output.String(), want)
	}
}

func TestFitReconnectStatusAvoidsWrapping(t *testing.T) {
	if got := fitReconnectStatus("123456789", 12); got != "123456" {
		t.Fatalf("fitReconnectStatus() = %q, want %q", got, "123456")
	}
	if got := fitReconnectStatus("短訊息", 80); got != "短訊息" {
		t.Fatalf("fitReconnectStatus() = %q, want original message", got)
	}
}

func TestAskpassFrameRoundTrip(t *testing.T) {
	var buffer bytes.Buffer
	if err := writeAskpassFrame(&buffer, []byte("Enter passphrase: ")); err != nil {
		t.Fatal(err)
	}
	got, err := readAskpassFrame(&buffer)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "Enter passphrase: " {
		t.Fatalf("askpass payload = %q", got)
	}
}

func TestAskpassBridgeRoundTrip(t *testing.T) {
	bridge, err := newAskpassBridge()
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.close()

	go func() {
		_ = writeAskpassFrame(bridge.childRequestWriter, []byte("SSH passphrase: "))
	}()
	select {
	case prompt := <-bridge.requests:
		if prompt != "SSH passphrase: " {
			t.Fatalf("askpass prompt = %q", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for askpass prompt")
	}
	if err := bridge.respond("secret"); err != nil {
		t.Fatal(err)
	}
	got, err := readAskpassFrame(bridge.childResponseReader)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "secret" {
		t.Fatalf("askpass response = %q", got)
	}
}

func TestReadPassphraseConsumesRawInputWithoutEchoingSecret(t *testing.T) {
	input := make(chan []byte, 2)
	inputClosed := make(chan struct{})
	interrupts := make(chan os.Signal)
	input <- []byte("sec")
	input <- []byte("ret\r")

	var prompt bytes.Buffer
	passphrase, err := readPassphrase(input, inputClosed, interrupts, &prompt, "Enter passphrase: ")
	if err != nil {
		t.Fatal(err)
	}
	if passphrase != "secret" {
		t.Fatalf("passphrase = %q, want secret", passphrase)
	}
	if prompt.String() != "Enter passphrase: \r\n" {
		t.Fatalf("prompt output = %q, secret was echoed", prompt.String())
	}
}
