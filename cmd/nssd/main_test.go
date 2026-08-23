package main

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/gaborltd/nss/internal/protocol"
	"github.com/gaborltd/nss/internal/session"
)

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
