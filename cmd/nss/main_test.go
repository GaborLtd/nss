package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestRemoteAttachCommandAddsUserLocalBin(t *testing.T) {
	if !strings.Contains(remoteAttachCommand, `PATH="$HOME/.local/bin:$PATH"`) {
		t.Fatalf("remote attach command = %q, expected ~/.local/bin in PATH", remoteAttachCommand)
	}
	if !strings.HasSuffix(remoteAttachCommand, "exec nssd attach") {
		t.Fatalf("remote attach command = %q, expected nssd attach", remoteAttachCommand)
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

func TestReconnectStatusPreservesCursorWithANSI(t *testing.T) {
	var output bytes.Buffer
	if err := writeReconnectStatus(&output, "retrying in 1s: ssh transport disconnected", true); err != nil {
		t.Fatalf("writeReconnectStatus() error = %v", err)
	}
	want := "\x1b7\r\n\x1b[2K[nss] retrying in 1s: ssh transport disconnected\r\n\x1b8"
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
	want := "\x1b7\r\n\x1b[2K\x1b8"
	if output.String() != want {
		t.Fatalf("clear output = %q, want %q", output.String(), want)
	}
}
