package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/gaborltd/nss/internal/protocol"
)

const (
	defaultMaxSpoolBytes = 4 << 20
	defaultRows          = 24
	defaultCols          = 80
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrAlreadyAttached = errors.New("session already attached")
	ErrSessionClosed   = errors.New("session is closed")
)

type Config struct {
	StateDir      string
	HomeDir       string
	DefaultShell  string
	MaxSpoolBytes int64
	InitialRows   uint16
	InitialCols   uint16
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	cfg      Config
}

type Session struct {
	manager   *Manager
	id        string
	secret    string
	spoolPath string
	cmd       *exec.Cmd
	ptmx      *os.File

	mu       sync.Mutex
	attached *Attachment
	closed   bool
}

type Attachment struct {
	session   *Session
	frames    chan protocol.Frame
	closeOnce sync.Once
}

type Info struct {
	ID       string
	Attached bool
	Closed   bool
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()
	for _, s := range sessions {
		s.close()
	}
}

func NewManager(cfg Config) (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	if cfg.HomeDir == "" {
		cfg.HomeDir = home
	}
	if cfg.StateDir == "" {
		cfg.StateDir = filepath.Join(home, ".local", "state", "nss")
	}
	if cfg.DefaultShell == "" {
		cfg.DefaultShell = os.Getenv("SHELL")
		if cfg.DefaultShell == "" {
			cfg.DefaultShell = "/bin/sh"
		}
	}
	if cfg.MaxSpoolBytes <= 0 {
		cfg.MaxSpoolBytes = defaultMaxSpoolBytes
	}
	if cfg.InitialRows == 0 {
		cfg.InitialRows = defaultRows
	}
	if cfg.InitialCols == 0 {
		cfg.InitialCols = defaultCols
	}
	if err := os.MkdirAll(filepath.Join(cfg.StateDir, "sessions"), 0700); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	return &Manager{sessions: make(map[string]*Session), cfg: cfg}, nil
}

func (m *Manager) Attach(req protocol.OpenRequest) (*Attachment, []byte, error) {
	if !validID(req.SessionID) {
		return nil, nil, errors.New("invalid session id")
	}
	if req.Secret == "" {
		return nil, nil, errors.New("missing session secret")
	}
	if req.Rows == 0 {
		req.Rows = m.cfg.InitialRows
	}
	if req.Cols == 0 {
		req.Cols = m.cfg.InitialCols
	}

	m.mu.Lock()
	s := m.sessions[req.SessionID]
	if s == nil {
		var err error
		s, err = m.newSession(req)
		if err != nil {
			m.mu.Unlock()
			return nil, nil, err
		}
		m.sessions[req.SessionID] = s
	}
	m.mu.Unlock()

	return s.attach(req)
}

func (m *Manager) List() []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]Info, 0, len(m.sessions))
	for _, s := range m.sessions {
		s.mu.Lock()
		items = append(items, Info{ID: s.id, Attached: s.attached != nil, Closed: s.closed})
		s.mu.Unlock()
	}
	return items
}

func (m *Manager) Close(id string) error {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s == nil {
		return ErrSessionNotFound
	}
	s.close()
	return nil
}

func (m *Manager) newSession(req protocol.OpenRequest) (*Session, error) {
	shell := req.Shell
	if shell == "" {
		shell = m.cfg.DefaultShell
	}
	cmd := exec.Command(shell)
	cmd.Dir = m.cfg.HomeDir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "NSS_SESSION_ID="+req.SessionID)

	dir := filepath.Join(m.cfg.StateDir, "sessions", req.SessionID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: req.Rows, Cols: req.Cols})
	if err != nil {
		return nil, fmt.Errorf("start shell PTY: %w", err)
	}

	s := &Session{
		manager:   m,
		id:        req.SessionID,
		secret:    req.Secret,
		spoolPath: filepath.Join(dir, "output.log"),
		cmd:       cmd,
		ptmx:      ptmx,
	}
	go s.readPTY()
	go func() {
		_ = cmd.Wait()
		s.finish()
	}()
	return s, nil
}

func (s *Session) attach(req protocol.OpenRequest) (*Attachment, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, nil, ErrSessionClosed
	}
	if subtle.ConstantTimeCompare([]byte(s.secret), []byte(req.Secret)) != 1 {
		return nil, nil, errors.New("invalid session secret")
	}
	if s.attached != nil {
		return nil, nil, ErrAlreadyAttached
	}

	replay, err := s.readAndClearSpoolLocked()
	if err != nil {
		return nil, nil, fmt.Errorf("read output spool: %w", err)
	}
	att := &Attachment{session: s, frames: make(chan protocol.Frame, 128)}
	s.attached = att
	return att, replay, nil
}

func (s *Session) readPTY() {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			s.emit(append([]byte(nil), buf[:n]...))
		}
		if err != nil {
			return
		}
	}
}

func (s *Session) emit(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.attached != nil {
		select {
		case s.attached.frames <- protocol.Frame{Type: protocol.TypeData, Payload: data}:
			return
		default:
		}
	}
	_ = s.appendSpoolLocked(data)
}

func (s *Session) WriteInput(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrSessionClosed
	}
	_, err := s.ptmx.Write(data)
	return err
}

func (s *Session) Resize(rows, cols uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrSessionClosed
	}
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

func (a *Attachment) Frames() <-chan protocol.Frame {
	return a.frames
}

func (a *Attachment) Detach() {
	a.session.mu.Lock()
	defer a.session.mu.Unlock()
	if a.session.attached == a {
		a.session.attached = nil
		a.closeLocked()
	}
}

func (a *Attachment) CloseSession() {
	a.session.close()
}

func (a *Attachment) WriteInput(data []byte) error {
	return a.session.WriteInput(data)
}

func (a *Attachment) Resize(rows, cols uint16) error {
	return a.session.Resize(rows, cols)
}

func (a *Attachment) closeLocked() {
	a.closeOnce.Do(func() {
		close(a.frames)
	})
}

func (s *Session) finish() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if s.attached != nil {
		select {
		case s.attached.frames <- protocol.Frame{Type: protocol.TypeClose}:
		default:
		}
		s.attached.closeLocked()
		s.attached = nil
	}
	_ = s.ptmx.Close()
	s.mu.Unlock()
	s.manager.remove(s)
	_ = os.RemoveAll(filepath.Dir(s.spoolPath))
}

func (s *Session) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if s.attached != nil {
		s.attached.closeLocked()
		s.attached = nil
	}
	_ = s.cmd.Process.Kill()
	_ = s.ptmx.Close()
	s.mu.Unlock()
	s.manager.remove(s)
	_ = os.RemoveAll(filepath.Dir(s.spoolPath))
}

func (m *Manager) remove(s *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions[s.id] == s {
		delete(m.sessions, s.id)
	}
}

func (s *Session) appendSpoolLocked(data []byte) error {
	if s.manager.cfg.MaxSpoolBytes <= 0 || len(data) == 0 {
		return nil
	}
	f, err := os.OpenFile(s.spoolPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	info, err := f.Stat()
	_ = f.Close()
	if err != nil || info.Size() <= s.manager.cfg.MaxSpoolBytes {
		return err
	}

	keep := s.manager.cfg.MaxSpoolBytes
	in, err := os.Open(s.spoolPath)
	if err != nil {
		return err
	}
	if _, err := in.Seek(-keep, io.SeekEnd); err != nil {
		_ = in.Close()
		return err
	}
	tail := make([]byte, int(keep))
	_, readErr := io.ReadFull(in, tail)
	_ = in.Close()
	if readErr != nil && readErr != io.ErrUnexpectedEOF {
		return readErr
	}
	tmp := s.spoolPath + ".tmp"
	if err := os.WriteFile(tmp, tail, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.spoolPath)
}

func (s *Session) readAndClearSpoolLocked() ([]byte, error) {
	data, err := os.ReadFile(s.spoolPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := os.Truncate(s.spoolPath, 0); err != nil {
		return nil, err
	}
	return data, nil
}

func validID(id string) bool {
	if id == "" || len(id) > 128 || strings.ContainsAny(id, "/\\") {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func RandomToken(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
