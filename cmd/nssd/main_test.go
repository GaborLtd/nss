package main

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/gaborltd/nss/internal/protocol"
	"github.com/gaborltd/nss/internal/session"
)

func TestRunServeReadyMessageUsesConfiguredPaths(t *testing.T) {
	// 這個測試確認 daemon ready 訊息包含實際設定，方便手動診斷。
	socketPath := "/tmp/nss-test/nssd.sock"
	stateDir := "/tmp/nss-test/state"
	maxSpoolMB := int64(8)

	message := formatReadyMessage(socketPath, stateDir, maxSpoolMB)
	for _, expected := range []string{socketPath, stateDir, "8 MiB", "nssd: ready"} {
		if !bytes.Contains([]byte(message), []byte(expected)) {
			t.Fatalf("ready message = %q, missing %q", message, expected)
		}
	}
}

func TestHandleAttachRoundTrip(t *testing.T) {
	m, err := session.NewManager(session.Config{
		StateDir:      t.TempDir(),
		DefaultShell:  "/bin/sh",
		MaxSpoolBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.CloseAll()

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go handleAttach(serverConn, m)
	_ = clientConn.SetDeadline(time.Now().Add(3 * time.Second))

	open, err := protocol.EncodeOpen(protocol.OpenRequest{
		SessionID: "handler-test",
		Secret:    "handler-secret",
		Rows:      24,
		Cols:      80,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := protocol.Write(clientConn, open); err != nil {
		t.Fatal(err)
	}
	ack, err := protocol.Read(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	if ack.Type != protocol.TypeOpenOK {
		t.Fatalf("open response type = %d, want %d", ack.Type, protocol.TypeOpenOK)
	}

	if err := protocol.Write(clientConn, protocol.Frame{
		Type:    protocol.TypeData,
		Payload: []byte("printf nss_handler_output; exit\n"),
	}); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	for {
		frame, err := protocol.Read(clientConn)
		if err != nil {
			t.Fatal(err)
		}
		if frame.Type == protocol.TypeData {
			output.Write(frame.Payload)
		}
		if frame.Type == protocol.TypeClose {
			break
		}
	}
	if !bytes.Contains(output.Bytes(), []byte("nss_handler_output")) {
		t.Fatalf("output = %q, expected handler marker", output.String())
	}
}

func TestHandleAdminListAndClose(t *testing.T) {
	m, err := session.NewManager(session.Config{StateDir: t.TempDir(), DefaultShell: "/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	defer m.CloseAll()
	attachment, _, err := m.Attach(protocol.OpenRequest{SessionID: "admin-session", Secret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer attachment.Detach()

	serverConn, clientConn := net.Pipe()
	go handleAttach(serverConn, m)
	_ = clientConn.SetDeadline(time.Now().Add(3 * time.Second))
	if err := protocol.Write(clientConn, protocol.Frame{Type: protocol.TypeAdminList}); err != nil {
		t.Fatal(err)
	}
	listFrame, err := protocol.Read(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	items, err := protocol.DecodeSessionInfo(listFrame)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].SessionID != "admin-session" || !items[0].Attached {
		t.Fatalf("admin list = %#v", items)
	}
	_ = clientConn.Close()

	serverConn, clientConn = net.Pipe()
	go handleAttach(serverConn, m)
	_ = clientConn.SetDeadline(time.Now().Add(3 * time.Second))
	closeFrame, err := protocol.EncodeAdminClose(protocol.AdminCloseRequest{SessionID: "admin-session"})
	if err != nil {
		t.Fatal(err)
	}
	if err := protocol.Write(clientConn, closeFrame); err != nil {
		t.Fatal(err)
	}
	result, err := protocol.Read(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != protocol.TypeAdminOK {
		t.Fatalf("admin close response type = %d, want %d", result.Type, protocol.TypeAdminOK)
	}
	_ = clientConn.Close()
	if len(m.List()) != 0 {
		t.Fatalf("sessions after admin close = %#v, want empty", m.List())
	}
}
