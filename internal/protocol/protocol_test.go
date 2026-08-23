package protocol

import (
	"bytes"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var wire bytes.Buffer
	want := Frame{Type: TypeData, Payload: []byte("hello")}
	if err := Write(&wire, want); err != nil {
		t.Fatal(err)
	}
	got, err := Read(&wire)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != want.Type || !bytes.Equal(got.Payload, want.Payload) {
		t.Fatalf("Read() = %#v, want %#v", got, want)
	}
}

func TestFrameTooLarge(t *testing.T) {
	payload := make([]byte, MaxFrameSize+1)
	if err := Write(&bytes.Buffer{}, Frame{Type: TypeData, Payload: payload}); err != ErrFrameTooLarge {
		t.Fatalf("Write() error = %v, want %v", err, ErrFrameTooLarge)
	}
}

func TestOpenRoundTrip(t *testing.T) {
	want := OpenRequest{SessionID: "session-1", Secret: "secret", Rows: 24, Cols: 80}
	frame, err := EncodeOpen(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeOpen(frame)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("DecodeOpen() = %#v, want %#v", got, want)
	}
}
