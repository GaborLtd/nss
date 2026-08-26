package main

import (
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
