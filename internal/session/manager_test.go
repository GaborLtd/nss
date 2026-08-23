package session

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gaborltd/nss/internal/protocol"
)

func TestAttachRunsShellAndReturnsOutput(t *testing.T) {
	m, err := NewManager(Config{
		StateDir:      t.TempDir(),
		DefaultShell:  "/bin/sh",
		MaxSpoolBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.CloseAll()

	att, _, err := m.Attach(protocol.OpenRequest{
		SessionID: "test-session",
		Secret:    "test-secret",
		Rows:      24,
		Cols:      80,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer att.Detach()

	if err := att.WriteInput([]byte("printf nss_test_output; exit\n")); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	deadline := time.After(3 * time.Second)
	for {
		select {
		case frame, ok := <-att.Frames():
			if !ok || frame.Type == protocol.TypeClose {
				if !bytes.Contains(output.Bytes(), []byte("nss_test_output")) {
					t.Fatalf("shell output = %q, expected test marker", output.String())
				}
				return
			}
			if frame.Type == protocol.TypeData {
				output.Write(frame.Payload)
			}
		case <-deadline:
			t.Fatalf("timed out waiting for shell output; got %q", output.String())
		}
	}
}

func TestAttachRejectsSecondClient(t *testing.T) {
	m, err := NewManager(Config{StateDir: t.TempDir(), DefaultShell: "/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	defer m.CloseAll()

	first, _, err := m.Attach(protocol.OpenRequest{SessionID: "same", Secret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Detach()
	if _, _, err := m.Attach(protocol.OpenRequest{SessionID: "same", Secret: "secret"}); err != ErrAlreadyAttached {
		t.Fatalf("second Attach() error = %v, want %v", err, ErrAlreadyAttached)
	}
}

func TestSpoolIsBoundedAndReplayed(t *testing.T) {
	stateDir := t.TempDir()
	m, err := NewManager(Config{StateDir: stateDir, MaxSpoolBytes: 8})
	if err != nil {
		t.Fatal(err)
	}
	s := &Session{
		manager:   m,
		spoolPath: filepath.Join(stateDir, "sessions", "spool", "output.log"),
	}
	if err := os.MkdirAll(filepath.Dir(s.spoolPath), 0700); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	err = s.appendSpoolLocked([]byte("0123456789abcdef"))
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(s.spoolPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 8 {
		t.Fatalf("spool size = %d, want 8", info.Size())
	}
	s.mu.Lock()
	replay, err := s.readAndClearSpoolLocked()
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if string(replay) != "89abcdef" {
		t.Fatalf("replay = %q, want %q", replay, "89abcdef")
	}
}
