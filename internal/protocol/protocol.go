package protocol

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	// MaxFrameSize 防止遠端送入異常大的 frame 造成無界記憶體配置。
	MaxFrameSize = 16 << 20
	headerSize   = 5
)

type Type byte

const (
	TypeOpen       Type = 1
	TypeOpenOK     Type = 2
	TypeOpenError  Type = 3
	TypeData       Type = 4
	TypeResize     Type = 5
	TypeClose      Type = 6
	TypeAdminList  Type = 7
	TypeAdminOK    Type = 8
	TypeAdminClose Type = 9
)

var ErrFrameTooLarge = errors.New("protocol frame too large")

type Frame struct {
	Type    Type
	Payload []byte
}

type OpenRequest struct {
	SessionID string `json:"session_id"`
	Secret    string `json:"secret"`
	Rows      uint16 `json:"rows"`
	Cols      uint16 `json:"cols"`
	Shell     string `json:"shell,omitempty"`
}

type OpenResponse struct {
	SessionID string `json:"session_id"`
	Replayed  int64  `json:"replayed_bytes"`
}

type AdminCloseRequest struct {
	SessionID string `json:"session_id"`
}

type SessionInfo struct {
	SessionID string `json:"session_id"`
	Attached  bool   `json:"attached"`
}

func EncodeAdminClose(req AdminCloseRequest) (Frame, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return Frame{}, fmt.Errorf("encode admin close request: %w", err)
	}
	return Frame{Type: TypeAdminClose, Payload: payload}, nil
}

func DecodeAdminClose(frame Frame) (AdminCloseRequest, error) {
	if frame.Type != TypeAdminClose {
		return AdminCloseRequest{}, fmt.Errorf("expected admin-close frame, got %d", frame.Type)
	}
	var req AdminCloseRequest
	if err := json.Unmarshal(frame.Payload, &req); err != nil {
		return AdminCloseRequest{}, fmt.Errorf("decode admin close request: %w", err)
	}
	return req, nil
}

func EncodeSessionInfo(items []SessionInfo) (Frame, error) {
	payload, err := json.Marshal(items)
	if err != nil {
		return Frame{}, fmt.Errorf("encode session list: %w", err)
	}
	return Frame{Type: TypeAdminOK, Payload: payload}, nil
}

func DecodeSessionInfo(frame Frame) ([]SessionInfo, error) {
	if frame.Type != TypeAdminOK {
		return nil, fmt.Errorf("expected admin-ok frame, got %d", frame.Type)
	}
	var items []SessionInfo
	if err := json.Unmarshal(frame.Payload, &items); err != nil {
		return nil, fmt.Errorf("decode session list: %w", err)
	}
	return items, nil
}

func EncodeOpen(req OpenRequest) (Frame, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return Frame{}, fmt.Errorf("encode open request: %w", err)
	}
	return Frame{Type: TypeOpen, Payload: payload}, nil
}

func DecodeOpen(frame Frame) (OpenRequest, error) {
	if frame.Type != TypeOpen {
		return OpenRequest{}, fmt.Errorf("expected open frame, got %d", frame.Type)
	}
	var req OpenRequest
	if err := json.Unmarshal(frame.Payload, &req); err != nil {
		return OpenRequest{}, fmt.Errorf("decode open request: %w", err)
	}
	return req, nil
}

func EncodeOpenResponse(resp OpenResponse) (Frame, error) {
	payload, err := json.Marshal(resp)
	if err != nil {
		return Frame{}, fmt.Errorf("encode open response: %w", err)
	}
	return Frame{Type: TypeOpenOK, Payload: payload}, nil
}

func DecodeOpenResponse(frame Frame) (OpenResponse, error) {
	if frame.Type != TypeOpenOK {
		return OpenResponse{}, fmt.Errorf("expected open-ok frame, got %d", frame.Type)
	}
	var resp OpenResponse
	if err := json.Unmarshal(frame.Payload, &resp); err != nil {
		return OpenResponse{}, fmt.Errorf("decode open response: %w", err)
	}
	return resp, nil
}

func Write(w io.Writer, frame Frame) error {
	if len(frame.Payload) > MaxFrameSize {
		return ErrFrameTooLarge
	}
	header := [headerSize]byte{byte(frame.Type)}
	binary.BigEndian.PutUint32(header[1:], uint32(len(frame.Payload)))
	if err := writeAll(w, header[:]); err != nil {
		return err
	}
	return writeAll(w, frame.Payload)
}

func Read(r io.Reader) (Frame, error) {
	var header [headerSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Frame{}, err
	}
	length := binary.BigEndian.Uint32(header[1:])
	if length > MaxFrameSize {
		return Frame{}, ErrFrameTooLarge
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return Frame{}, err
	}
	return Frame{Type: Type(header[0]), Payload: payload}, nil
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
