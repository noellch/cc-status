package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/anthropics/cc-status-go/pkg/model"
)

const (
	maxMessageSize = 65536           // 64KB
	clientTimeout  = 5 * time.Second // read deadline per client
)

// Server listens on a Unix domain socket and dispatches incoming
// SessionEvent messages to a callback.
type Server struct {
	socketPath string
	onEvent    func(model.SessionEvent)
	listener   net.Listener

	mu      sync.Mutex
	stopped bool

	wg sync.WaitGroup
}

// New creates a Server that will listen on socketPath and call onEvent
// for each valid incoming event.
func New(socketPath string, onEvent func(model.SessionEvent)) *Server {
	return &Server{
		socketPath: socketPath,
		onEvent:    onEvent,
	}
}

// Start begins accepting connections on the Unix socket.
func (s *Server) Start() error {
	dir := filepath.Dir(s.socketPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir socket dir: %w", err)
	}

	if err := s.cleanupStaleSocket(); err != nil {
		return err
	}

	// Set umask to restrict socket permissions, then restore.
	oldMask := syscall.Umask(0o077)
	ln, err := net.Listen("unix", s.socketPath)
	syscall.Umask(oldMask)
	if err != nil {
		return fmt.Errorf("listen unix: %w", err)
	}

	if err := os.Chmod(s.socketPath, 0o600); err != nil {
		ln.Close()
		return fmt.Errorf("chmod socket: %w", err)
	}

	s.listener = ln

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()

	if s.listener != nil {
		s.listener.Close()
	}

	s.wg.Wait()
	os.Remove(s.socketPath)
}

// cleanupStaleSocket checks if an existing socket file is in use.
// If a connection succeeds, another instance is running and we return an error.
// If the connection fails, we remove the stale file.
func (s *Server) cleanupStaleSocket() error {
	_, err := os.Stat(s.socketPath)
	if os.IsNotExist(err) {
		return nil
	}

	conn, err := net.DialTimeout("unix", s.socketPath, 1*time.Second)
	if err == nil {
		// Another instance is listening.
		conn.Close()
		return fmt.Errorf("another instance is already running on %s", s.socketPath)
	}

	// Stale socket file — remove it.
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			stopped := s.stopped
			s.mu.Unlock()
			if stopped {
				return
			}
			continue
		}
		s.wg.Add(1)
		go s.handleClient(conn)
	}
}

func (s *Server) handleClient(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(clientTimeout))

	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if len(buf) > maxMessageSize {
				// Message too large — drop the connection.
				return
			}
		}
		if err != nil {
			if err == io.EOF {
				break // clean close, process the message
			}
			return // read error or timeout
		}
	}

	if len(buf) == 0 {
		return
	}

	var event model.SessionEvent
	if err := json.Unmarshal(buf, &event); err != nil {
		return
	}

	s.onEvent(event)
}
