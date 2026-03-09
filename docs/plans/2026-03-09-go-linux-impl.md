# Go/Linux Port Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go-based Linux system tray app and cross-platform hook binary that speaks the same Unix socket protocol as the existing Swift macOS version.

**Architecture:** Two Go binaries in a single module under `cc-status-go/`. The hook binary reads Claude Code stdin JSON, maps to SessionEvent, sends to socket. The tray binary listens on socket, tracks sessions, displays status via fyne-io/systray with emoji indicators.

**Tech Stack:** Go 1.22+, fyne-io/systray, embed (for icon assets), encoding/json, net (Unix socket), os/exec (terminal focus, git branch)

---

### Task 1: Go Module + Shared Models

**Files:**
- Create: `cc-status-go/go.mod`
- Create: `cc-status-go/pkg/model/model.go`
- Create: `cc-status-go/pkg/model/model_test.go`

**Step 1: Initialize Go module**

```bash
mkdir -p cc-status-go/pkg/model
cd cc-status-go
go mod init github.com/anthropics/cc-status-go
```

**Step 2: Write model test**

```go
// pkg/model/model_test.go
package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSessionEventJSON(t *testing.T) {
	e := SessionEvent{
		SessionID:  "abc123",
		Event:      StatusActive,
		Cwd:        "/home/user/project",
		Branch:     "main",
		Summary:    "Working...",
		TerminalID: strPtr("ghostty:win-1"),
		Timestamp:  1741520400.0,
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}

	// Verify snake_case keys match Swift protocol
	var raw map[string]any
	json.Unmarshal(data, &raw)

	if _, ok := raw["session_id"]; !ok {
		t.Error("expected snake_case key session_id")
	}
	if _, ok := raw["terminal_id"]; !ok {
		t.Error("expected snake_case key terminal_id")
	}

	// Round-trip
	var decoded SessionEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SessionID != "abc123" {
		t.Errorf("got %q, want abc123", decoded.SessionID)
	}
	if decoded.Event != StatusActive {
		t.Errorf("got %q, want active", decoded.Event)
	}
}

func TestSessionEventNilTerminalID(t *testing.T) {
	e := SessionEvent{
		SessionID: "x",
		Event:     StatusRemove,
		Cwd:       "/tmp",
		Timestamp: 1000.0,
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	// terminal_id should be null or absent
	var raw map[string]any
	json.Unmarshal(data, &raw)
	if v, ok := raw["terminal_id"]; ok && v != nil {
		t.Errorf("expected null terminal_id, got %v", v)
	}
}

func strPtr(s string) *string { return &s }
```

**Step 3: Run test, verify it fails**

```bash
cd cc-status-go && go test ./pkg/model/ -v
```

Expected: FAIL (types not defined)

**Step 4: Write model implementation**

```go
// pkg/model/model.go
package model

import (
	"os"
	"path/filepath"
)

type SessionStatus string

const (
	StatusActive  SessionStatus = "active"
	StatusWaiting SessionStatus = "waiting"
	StatusDone    SessionStatus = "done"
	StatusRemove  SessionStatus = "remove"
)

type SessionEvent struct {
	SessionID  string        `json:"session_id"`
	Event      SessionStatus `json:"event"`
	Cwd        string        `json:"cwd"`
	Branch     string        `json:"branch"`
	Summary    string        `json:"summary"`
	TerminalID *string       `json:"terminal_id"`
	Timestamp  float64       `json:"timestamp"`
}

type SessionInfo struct {
	SessionID  string        `json:"session_id"`
	Status     SessionStatus `json:"status"`
	Cwd        string        `json:"cwd"`
	Branch     string        `json:"branch"`
	Summary    string        `json:"summary"`
	TerminalID *string       `json:"terminal_id"`
	LastUpdated float64      `json:"last_updated"`
}

func SocketDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cc-status")
}

func SocketPath() string {
	return filepath.Join(SocketDir(), "cc-status.sock")
}

func SessionsPath() string {
	return filepath.Join(SocketDir(), "sessions.json")
}
```

**Step 5: Run test, verify it passes**

```bash
cd cc-status-go && go test ./pkg/model/ -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add cc-status-go/
git commit -m "feat(go): add Go module with shared models and JSON protocol tests"
```

---

### Task 2: Session Store

**Files:**
- Create: `cc-status-go/internal/session/store.go`
- Create: `cc-status-go/internal/session/store_test.go`

**Step 1: Write store test**

```go
// internal/session/store_test.go
package session

import (
	"testing"
	"time"

	"github.com/anthropics/cc-status-go/pkg/model"
)

func TestHandleEvent(t *testing.T) {
	s := NewStore()
	now := float64(time.Now().Unix())

	s.HandleEvent(model.SessionEvent{
		SessionID: "s1",
		Event:     model.StatusActive,
		Cwd:       "/tmp/project",
		Branch:    "main",
		Summary:   "Working...",
		Timestamp: now,
	})

	sessions := s.All()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions["s1"].Status != model.StatusActive {
		t.Errorf("expected active, got %s", sessions["s1"].Status)
	}
}

func TestHandleEventRemove(t *testing.T) {
	s := NewStore()
	now := float64(time.Now().Unix())

	s.HandleEvent(model.SessionEvent{SessionID: "s1", Event: model.StatusActive, Cwd: "/tmp", Timestamp: now})
	s.HandleEvent(model.SessionEvent{SessionID: "s1", Event: model.StatusRemove, Cwd: "/tmp", Timestamp: now})

	if len(s.All()) != 0 {
		t.Error("expected 0 sessions after remove")
	}
}

func TestCleanupStale(t *testing.T) {
	s := NewStore()
	old := float64(time.Now().Add(-40 * time.Minute).Unix())
	recent := float64(time.Now().Unix())

	s.HandleEvent(model.SessionEvent{SessionID: "stale", Event: model.StatusWaiting, Cwd: "/tmp", Timestamp: old})
	s.HandleEvent(model.SessionEvent{SessionID: "fresh", Event: model.StatusWaiting, Cwd: "/tmp", Timestamp: recent})

	s.CleanupStale()

	sessions := s.All()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if _, ok := sessions["fresh"]; !ok {
		t.Error("expected fresh session to survive cleanup")
	}
}

func TestSortedSessions(t *testing.T) {
	s := NewStore()
	now := float64(time.Now().Unix())

	s.HandleEvent(model.SessionEvent{SessionID: "a", Event: model.StatusActive, Cwd: "/tmp", Timestamp: now})
	s.HandleEvent(model.SessionEvent{SessionID: "w", Event: model.StatusWaiting, Cwd: "/tmp", Timestamp: now})
	s.HandleEvent(model.SessionEvent{SessionID: "d", Event: model.StatusDone, Cwd: "/tmp", Timestamp: now})

	sorted := s.Sorted()
	if len(sorted) != 3 {
		t.Fatalf("expected 3, got %d", len(sorted))
	}
	// waiting first, then done, then active
	if sorted[0].Status != model.StatusWaiting {
		t.Errorf("first should be waiting, got %s", sorted[0].Status)
	}
	if sorted[1].Status != model.StatusDone {
		t.Errorf("second should be done, got %s", sorted[1].Status)
	}
	if sorted[2].Status != model.StatusActive {
		t.Errorf("third should be active, got %s", sorted[2].Status)
	}
}

func TestDismissAll(t *testing.T) {
	s := NewStore()
	now := float64(time.Now().Unix())
	s.HandleEvent(model.SessionEvent{SessionID: "s1", Event: model.StatusActive, Cwd: "/tmp", Timestamp: now})
	s.HandleEvent(model.SessionEvent{SessionID: "s2", Event: model.StatusWaiting, Cwd: "/tmp", Timestamp: now})

	s.DismissAll()
	if len(s.All()) != 0 {
		t.Error("expected 0 sessions after dismiss all")
	}
}
```

**Step 2: Run test, verify it fails**

```bash
cd cc-status-go && go test ./internal/session/ -v
```

**Step 3: Write store implementation**

```go
// internal/session/store.go
package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/anthropics/cc-status-go/pkg/model"
)

type Store struct {
	mu       sync.RWMutex
	sessions map[string]model.SessionInfo
	onChange func() // called after mutation, under no lock

	saveTimer *time.Timer
	saveMu    sync.Mutex
}

func NewStore() *Store {
	return &Store{
		sessions: make(map[string]model.SessionInfo),
	}
}

func (s *Store) SetOnChange(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = fn
}

func (s *Store) HandleEvent(e model.SessionEvent) {
	s.mu.Lock()
	if e.Event == model.StatusRemove {
		delete(s.sessions, e.SessionID)
	} else {
		s.sessions[e.SessionID] = model.SessionInfo{
			SessionID:   e.SessionID,
			Status:      e.Event,
			Cwd:         e.Cwd,
			Branch:      e.Branch,
			Summary:     e.Summary,
			TerminalID:  e.TerminalID,
			LastUpdated: e.Timestamp,
		}
	}
	onChange := s.onChange
	s.mu.Unlock()

	if onChange != nil {
		onChange()
	}
	s.scheduleSave()
}

func (s *Store) All() map[string]model.SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]model.SessionInfo, len(s.sessions))
	for k, v := range s.sessions {
		cp[k] = v
	}
	return cp
}

func (s *Store) Sorted() []model.SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.SessionInfo, 0, len(s.sessions))
	for _, v := range s.sessions {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool {
		oi, oj := statusOrder(list[i].Status), statusOrder(list[j].Status)
		if oi != oj {
			return oi < oj
		}
		return list[i].LastUpdated > list[j].LastUpdated
	})
	return list
}

func statusOrder(s model.SessionStatus) int {
	switch s {
	case model.StatusWaiting:
		return 0
	case model.StatusDone:
		return 1
	case model.StatusActive:
		return 2
	default:
		return 3
	}
}

func (s *Store) WaitingCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, v := range s.sessions {
		if v.Status == model.StatusWaiting {
			n++
		}
	}
	return n
}

func (s *Store) DoneCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, v := range s.sessions {
		if v.Status == model.StatusDone {
			n++
		}
	}
	return n
}

func (s *Store) DismissAll() {
	s.mu.Lock()
	s.sessions = make(map[string]model.SessionInfo)
	onChange := s.onChange
	s.mu.Unlock()
	if onChange != nil {
		onChange()
	}
	s.scheduleSave()
}

func (s *Store) CleanupStale() {
	s.mu.Lock()
	now := time.Now()
	idleThreshold := now.Add(-30 * time.Minute)
	activeThreshold := now.Add(-10 * time.Minute)
	changed := false

	for id, sess := range s.sessions {
		updated := time.Unix(int64(sess.LastUpdated), 0)
		switch sess.Status {
		case model.StatusWaiting, model.StatusDone:
			if updated.Before(idleThreshold) {
				delete(s.sessions, id)
				changed = true
			}
		case model.StatusActive:
			if updated.Before(activeThreshold) {
				delete(s.sessions, id)
				changed = true
			}
		case model.StatusRemove:
			delete(s.sessions, id)
			changed = true
		}
	}
	onChange := s.onChange
	s.mu.Unlock()

	if changed {
		if onChange != nil {
			onChange()
		}
		s.scheduleSave()
	}
}

// Persistence

func (s *Store) LoadFromDisk() {
	path := model.SessionsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var saved map[string]model.SessionInfo
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}
	s.mu.Lock()
	s.sessions = saved
	s.mu.Unlock()
	s.CleanupStale()
}

func (s *Store) scheduleSave() {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.saveTimer = time.AfterFunc(1*time.Second, func() {
		s.saveToDisk()
	})
}

func (s *Store) saveToDisk() {
	s.mu.RLock()
	data, err := json.Marshal(s.sessions)
	s.mu.RUnlock()
	if err != nil {
		return
	}
	dir := model.SocketDir()
	os.MkdirAll(dir, 0o700)

	tmp := model.SessionsPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	os.Rename(tmp, model.SessionsPath())
}
```

**Step 4: Run test, verify it passes**

```bash
cd cc-status-go && go test ./internal/session/ -v
```

**Step 5: Commit**

```bash
git add cc-status-go/
git commit -m "feat(go): add session store with cleanup, persistence, and sorting"
```

---

### Task 3: Socket Server

**Files:**
- Create: `cc-status-go/internal/server/server.go`
- Create: `cc-status-go/internal/server/server_test.go`

**Step 1: Write server test**

```go
// internal/server/server_test.go
package server

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropics/cc-status-go/pkg/model"
)

func TestServerReceivesEvent(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	received := make(chan model.SessionEvent, 1)
	srv := New(sockPath, func(e model.SessionEvent) {
		received <- e
	})

	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	// Give server time to start listening
	time.Sleep(50 * time.Millisecond)

	// Send an event
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	event := model.SessionEvent{
		SessionID: "test-1",
		Event:     model.StatusActive,
		Cwd:       "/tmp",
		Branch:    "main",
		Summary:   "Working...",
		Timestamp: float64(time.Now().Unix()),
	}
	data, _ := json.Marshal(event)
	conn.Write(data)
	conn.Close()

	select {
	case e := <-received:
		if e.SessionID != "test-1" {
			t.Errorf("got session_id %q, want test-1", e.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestServerRejectsOversizedMessage(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	received := make(chan model.SessionEvent, 1)
	srv := New(sockPath, func(e model.SessionEvent) {
		received <- e
	})
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()
	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	// Send 128KB of junk
	junk := make([]byte, 128*1024)
	conn.Write(junk)
	conn.Close()

	select {
	case <-received:
		t.Error("should not have received an event from oversized message")
	case <-time.After(200 * time.Millisecond):
		// Good — no event received
	}
}

func TestServerStaleSocketCleanup(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	// Create a stale socket file
	os.WriteFile(sockPath, []byte("stale"), 0o600)

	srv := New(sockPath, func(e model.SessionEvent) {})
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	// Server should have cleaned up and started successfully
	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatal("should be able to connect after stale cleanup:", err)
	}
	conn.Close()
}
```

**Step 2: Run test, verify it fails**

```bash
cd cc-status-go && go test ./internal/server/ -v
```

**Step 3: Write server implementation**

```go
// internal/server/server.go
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/anthropics/cc-status-go/pkg/model"
)

const (
	maxMessageSize = 65536 // 64 KB
	clientTimeout  = 5 * time.Second
)

type Server struct {
	socketPath string
	onEvent    func(model.SessionEvent)
	listener   net.Listener
	mu         sync.Mutex
	stopped    bool
	wg         sync.WaitGroup
}

func New(socketPath string, onEvent func(model.SessionEvent)) *Server {
	return &Server{
		socketPath: socketPath,
		onEvent:    onEvent,
	}
}

func (s *Server) Start() error {
	dir := filepath.Dir(s.socketPath)
	os.MkdirAll(dir, 0o700)

	if err := s.cleanupStaleSocket(); err != nil {
		return err
	}

	// Set umask before bind to restrict socket permissions
	oldUmask := syscall.Umask(0o077)
	ln, err := net.Listen("unix", s.socketPath)
	syscall.Umask(oldUmask)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	os.Chmod(s.socketPath, 0o600)

	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	s.wg.Add(1)
	go s.acceptLoop()

	log.Printf("[cc-status] Listening on %s", s.socketPath)
	return nil
}

func (s *Server) Stop() {
	s.mu.Lock()
	s.stopped = true
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.Unlock()

	s.wg.Wait()
	os.Remove(s.socketPath)
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

	data := make([]byte, 0, 4096)
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
			if len(data) > maxMessageSize {
				log.Printf("[cc-status] Client exceeded max message size, disconnecting")
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				// Timeout or other error — use whatever data we have
			}
			break
		}
	}

	if len(data) == 0 {
		return
	}

	var event model.SessionEvent
	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("[cc-status] Failed to decode event: %v", err)
		return
	}

	s.onEvent(event)
}

func (s *Server) cleanupStaleSocket() error {
	_, err := os.Stat(s.socketPath)
	if os.IsNotExist(err) {
		return nil
	}

	// Try to connect — if it succeeds, another instance is running
	conn, err := net.DialTimeout("unix", s.socketPath, 1*time.Second)
	if err != nil {
		// Connection failed — stale socket, safe to remove
		os.Remove(s.socketPath)
		log.Printf("[cc-status] Removed stale socket file")
		return nil
	}
	conn.Close()

	// Another instance is actively listening
	return fmt.Errorf("another instance is listening on %s", s.socketPath)
}
```

**Step 4: Run test, verify it passes**

```bash
cd cc-status-go && go test ./internal/server/ -v
```

**Step 5: Commit**

```bash
git add cc-status-go/
git commit -m "feat(go): add hardened Unix socket server with size limit and timeout"
```

---

### Task 4: Hook Binary

**Files:**
- Create: `cc-status-go/internal/hook/hook.go`
- Create: `cc-status-go/internal/hook/terminal.go`
- Create: `cc-status-go/internal/hook/installer.go`
- Create: `cc-status-go/internal/hook/hook_test.go`
- Create: `cc-status-go/cmd/cc-status-hook/main.go`

**Step 1: Write hook parsing test**

```go
// internal/hook/hook_test.go
package hook

import (
	"testing"

	"github.com/anthropics/cc-status-go/pkg/model"
)

func TestParseHookInput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSID string
		wantEvt model.SessionStatus
		wantSum string
		wantNil bool
	}{
		{
			name:    "SessionStart",
			input:   `{"hook_event_name":"SessionStart","session_id":"s1","cwd":"/tmp"}`,
			wantSID: "s1",
			wantEvt: model.StatusActive,
			wantSum: "Session started",
		},
		{
			name:    "UserPromptSubmit",
			input:   `{"hook_event_name":"UserPromptSubmit","session_id":"s2","cwd":"/tmp"}`,
			wantSID: "s2",
			wantEvt: model.StatusActive,
			wantSum: "Working...",
		},
		{
			name:    "Stop with message",
			input:   `{"hook_event_name":"Stop","session_id":"s3","cwd":"/tmp","last_assistant_message":"I fixed the bug"}`,
			wantSID: "s3",
			wantEvt: model.StatusWaiting,
			wantSum: "I fixed the bug",
		},
		{
			name:    "Notification permission",
			input:   `{"hook_event_name":"Notification","session_id":"s4","cwd":"/tmp","notification_type":"permission_prompt","message":"Needs approval"}`,
			wantSID: "s4",
			wantEvt: model.StatusWaiting,
			wantSum: "Needs approval",
		},
		{
			name:    "Notification other type is ignored",
			input:   `{"hook_event_name":"Notification","session_id":"s5","cwd":"/tmp","notification_type":"other"}`,
			wantNil: true,
		},
		{
			name:    "SessionEnd",
			input:   `{"hook_event_name":"SessionEnd","session_id":"s6","cwd":"/tmp"}`,
			wantSID: "s6",
			wantEvt: model.StatusRemove,
		},
		{
			name:    "Unknown event",
			input:   `{"hook_event_name":"FutureEvent","session_id":"s7","cwd":"/tmp"}`,
			wantNil: true,
		},
		{
			name:    "Empty input",
			input:   ``,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := ParseHookInput([]byte(tt.input))
			if tt.wantNil {
				if event != nil {
					t.Errorf("expected nil, got %+v", event)
				}
				return
			}
			if event == nil {
				t.Fatal("expected non-nil event")
			}
			if event.SessionID != tt.wantSID {
				t.Errorf("session_id: got %q, want %q", event.SessionID, tt.wantSID)
			}
			if event.Event != tt.wantEvt {
				t.Errorf("event: got %q, want %q", event.Event, tt.wantEvt)
			}
			if tt.wantSum != "" && event.Summary != tt.wantSum {
				t.Errorf("summary: got %q, want %q", event.Summary, tt.wantSum)
			}
		})
	}
}

func TestDetectTerminalID(t *testing.T) {
	// With GHOSTTY_BIN_DIR set
	env := map[string]string{"GHOSTTY_BIN_DIR": "/usr/bin", "GHOSTTY_WINDOW_ID": "42"}
	tid := DetectTerminalIDFromEnv(env)
	if tid == nil || *tid != "ghostty:42" {
		t.Errorf("expected ghostty:42, got %v", tid)
	}

	// With ITERM_SESSION_ID
	env = map[string]string{"ITERM_SESSION_ID": "w0t0p0:ABCD"}
	tid = DetectTerminalIDFromEnv(env)
	if tid == nil || *tid != "iterm:w0t0p0:ABCD" {
		t.Errorf("expected iterm:w0t0p0:ABCD, got %v", tid)
	}

	// Empty env
	env = map[string]string{}
	tid = DetectTerminalIDFromEnv(env)
	if tid != nil {
		t.Errorf("expected nil, got %v", *tid)
	}
}
```

**Step 2: Run test, verify it fails**

```bash
cd cc-status-go && go test ./internal/hook/ -v
```

**Step 3: Write hook implementation**

```go
// internal/hook/hook.go
package hook

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/cc-status-go/pkg/model"
)

func ParseHookInput(data []byte) *model.SessionEvent {
	if len(data) == 0 {
		return nil
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	hookEvent, _ := raw["hook_event_name"].(string)
	if hookEvent == "" {
		return nil
	}

	sessionID, _ := raw["session_id"].(string)
	if sessionID == "" {
		sessionID = fmt.Sprintf("unknown-%d", os.Getpid())
	}

	cwd, _ := raw["cwd"].(string)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	var status model.SessionStatus
	var summary string

	switch hookEvent {
	case "SessionStart":
		status = model.StatusActive
		summary = "Session started"
	case "UserPromptSubmit":
		status = model.StatusActive
		summary = "Working..."
	case "Stop":
		status = model.StatusWaiting
		if msg, ok := raw["last_assistant_message"].(string); ok && msg != "" {
			if len(msg) > 80 {
				summary = msg[:80] + "..."
			} else {
				summary = msg
			}
		} else {
			summary = "Waiting for input"
		}
	case "Notification":
		ntype, _ := raw["notification_type"].(string)
		if ntype == "permission_prompt" || ntype == "idle_prompt" {
			status = model.StatusWaiting
			summary, _ = raw["message"].(string)
			if summary == "" {
				summary = "Needs attention"
			}
		} else {
			return nil
		}
	case "SessionEnd":
		status = model.StatusRemove
		summary = ""
	default:
		return nil
	}

	env := os.Environ()
	envMap := make(map[string]string, len(env))
	for _, e := range env {
		if k, v, ok := strings.Cut(e, "="); ok {
			envMap[k] = v
		}
	}
	terminalID := DetectTerminalIDFromEnv(envMap)
	branch := GetCurrentBranch(cwd)

	return &model.SessionEvent{
		SessionID:  sessionID,
		Event:      status,
		Cwd:        cwd,
		Branch:     branch,
		Summary:    summary,
		TerminalID: terminalID,
		Timestamp:  float64(time.Now().Unix()),
	}
}

func GetCurrentBranch(cwd string) string {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func SendToSocket(event *model.SessionEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	conn, err := net.DialTimeout("unix", model.SocketPath(), 2*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	conn.Write(data)
}
```

```go
// internal/hook/terminal.go
package hook

func DetectTerminalIDFromEnv(env map[string]string) *string {
	if v, ok := env["ITERM_SESSION_ID"]; ok {
		s := "iterm:" + v
		return &s
	}
	if v, ok := env["TERM_SESSION_ID"]; ok {
		s := "terminal:" + v
		return &s
	}
	if v, ok := env["WARP_SESSION_ID"]; ok {
		s := "warp:" + v
		return &s
	}
	if _, ok := env["GHOSTTY_BIN_DIR"]; ok {
		wid := env["GHOSTTY_WINDOW_ID"]
		s := "ghostty:" + wid
		return &s
	}
	if env["TERM_PROGRAM"] == "ghostty" {
		wid := env["GHOSTTY_WINDOW_ID"]
		s := "ghostty:" + wid
		return &s
	}

	// Linux: TERM_PROGRAM detection
	termProgramMap := map[string]string{
		"WezTerm": "WezTerm", "zed": "Zed", "Hyper": "Hyper",
		"kitty": "kitty", "Alacritty": "Alacritty",
		"cursor": "Cursor", "vscode": "Visual Studio Code",
	}
	if tp, ok := env["TERM_PROGRAM"]; ok {
		if app, found := termProgramMap[tp]; found {
			s := "app:" + app
			return &s
		}
	}

	if v, ok := env["TTY"]; ok {
		s := "terminal:" + v
		return &s
	}
	if tp, ok := env["TERM_PROGRAM"]; ok && tp != "" {
		s := "app:" + tp
		return &s
	}

	return nil
}
```

```go
// internal/hook/installer.go
package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var hookEvents = []string{
	"SessionStart", "UserPromptSubmit", "Stop", "Notification", "SessionEnd",
}

const commandMarker = "cc-status-hook"

func Install() error {
	settingsPath := settingsFilePath()
	root, err := readOrCreateSettings(settingsPath)
	if err != nil {
		return err
	}

	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	hookPath := resolveHookPath()
	var added []string

	for _, event := range hookEvents {
		groups, _ := hooks[event].([]any)
		if containsOurHook(groups) {
			continue
		}
		newGroup := map[string]any{
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": hookPath,
					"async":   true,
					"timeout": 5,
				},
			},
		}
		groups = append(groups, newGroup)
		hooks[event] = groups
		added = append(added, event)
	}

	root["hooks"] = hooks
	if err := writeSettings(root, settingsPath); err != nil {
		return err
	}

	if len(added) == 0 {
		fmt.Println("cc-status hooks already installed in ~/.claude/settings.json")
	} else {
		fmt.Printf("Installed cc-status hooks for events: %s\n", strings.Join(added, ", "))
		fmt.Printf("Settings written to %s\n", settingsPath)
	}
	return nil
}

func Uninstall() error {
	settingsPath := settingsFilePath()
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No ~/.claude/settings.json found. Nothing to uninstall.")
			return nil
		}
		return err
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}

	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		fmt.Println("No hooks section found. Nothing to uninstall.")
		return nil
	}

	var removed []string
	for event := range hooks {
		groups, _ := hooks[event].([]any)
		var filtered []any
		didRemove := false
		for _, g := range groups {
			group, _ := g.(map[string]any)
			if group == nil {
				filtered = append(filtered, g)
				continue
			}
			hks, _ := group["hooks"].([]any)
			var kept []any
			for _, h := range hks {
				hook, _ := h.(map[string]any)
				if !isOurHook(hook) {
					kept = append(kept, h)
				} else {
					didRemove = true
				}
			}
			if len(kept) > 0 {
				group["hooks"] = kept
				filtered = append(filtered, group)
			}
		}
		if didRemove {
			removed = append(removed, event)
		}
		if len(filtered) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = filtered
		}
	}

	root["hooks"] = hooks
	if err := writeSettings(root, settingsPath); err != nil {
		return err
	}

	if len(removed) == 0 {
		fmt.Println("No cc-status hooks found. Nothing to uninstall.")
	} else {
		fmt.Printf("Removed cc-status hooks from events: %s\n", strings.Join(removed, ", "))
	}
	return nil
}

func settingsFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func resolveHookPath() string {
	exe, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe
	}
	return resolved
}

func readOrCreateSettings(path string) (map[string]any, error) {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0o700)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{"hooks": map[string]any{}}, nil
		}
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return map[string]any{"hooks": map[string]any{}}, nil
	}
	return root, nil
}

func writeSettings(root map[string]any, path string) error {
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}

	lockPath := path + ".lock"
	lockFD, err := syscall.Open(lockPath, syscall.O_CREAT|syscall.O_WRONLY, 0o644)
	if err == nil {
		defer func() {
			syscall.Flock(lockFD, syscall.LOCK_UN)
			syscall.Close(lockFD)
		}()
		syscall.Flock(lockFD, syscall.LOCK_EX)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func containsOurHook(groups []any) bool {
	for _, g := range groups {
		group, _ := g.(map[string]any)
		hks, _ := group["hooks"].([]any)
		for _, h := range hks {
			hook, _ := h.(map[string]any)
			if isOurHook(hook) {
				return true
			}
		}
	}
	return false
}

func isOurHook(hook map[string]any) bool {
	cmd, _ := hook["command"].(string)
	return strings.Contains(cmd, commandMarker)
}
```

**Step 4: Write hook main**

```go
// cmd/cc-status-hook/main.go
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/anthropics/cc-status-go/internal/hook"
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "install":
			if err := hook.Install(); err != nil {
				fmt.Fprintf(os.Stderr, "Error installing hooks: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "uninstall":
			if err := hook.Uninstall(); err != nil {
				fmt.Fprintf(os.Stderr, "Error uninstalling hooks: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	data, _ := io.ReadAll(os.Stdin)
	event := hook.ParseHookInput(data)
	if event == nil {
		os.Exit(0)
	}
	hook.SendToSocket(event)
}
```

**Step 5: Run all tests**

```bash
cd cc-status-go && go test ./... -v
```

**Step 6: Build hook binary**

```bash
cd cc-status-go && go build -o bin/cc-status-hook ./cmd/cc-status-hook
```

**Step 7: Commit**

```bash
git add cc-status-go/
git commit -m "feat(go): add hook binary with event parsing, terminal detection, and installer"
```

---

### Task 5: Tray Icon Assets

**Files:**
- Create: `cc-status-go/assets/gen_icons.go` (build-time generator, not committed output)
- Create: `cc-status-go/internal/tray/icons.go`

**Step 1: Generate tray icon PNGs programmatically**

Create a small Go program that generates 16x16 colored dot PNGs. Run it once, commit the output.

```go
// assets/gen_icons.go
//go:build ignore

package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	colors := map[string]color.RGBA{
		"idle":    {180, 180, 180, 255},
		"active":  {107, 143, 173, 255},
		"waiting": {232, 156, 77, 255},
		"done":    {122, 176, 110, 255},
	}
	for name, c := range colors {
		generateDot(name+".png", c)
	}
}

func generateDot(filename string, c color.RGBA) {
	size := 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	cx, cy, r := float64(size)/2, float64(size)/2, float64(size)/2-2

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist <= r {
				// Anti-aliased edge
				alpha := 1.0
				if dist > r-1 {
					alpha = r - dist + 1
				}
				img.SetRGBA(x, y, color.RGBA{c.R, c.G, c.B, uint8(float64(c.A) * alpha)})
			}
		}
	}

	f, _ := os.Create(filename)
	defer f.Close()
	png.Encode(f, img)
}
```

```bash
cd cc-status-go/assets && go run gen_icons.go
```

**Step 2: Embed icons**

```go
// internal/tray/icons.go
package tray

import (
	_ "embed"
)

var (
	//go:embed ../../assets/idle.png
	iconIdle []byte
	//go:embed ../../assets/active.png
	iconActive []byte
	//go:embed ../../assets/waiting.png
	iconWaiting []byte
	//go:embed ../../assets/done.png
	iconDone []byte
)
```

**Step 3: Commit**

```bash
git add cc-status-go/assets/ cc-status-go/internal/tray/icons.go
git commit -m "feat(go): add tray icon assets (colored dot PNGs)"
```

---

### Task 6: Tray UI + Main Binary

**Files:**
- Create: `cc-status-go/internal/tray/tray.go`
- Create: `cc-status-go/internal/tray/focus.go`
- Create: `cc-status-go/cmd/cc-status-tray/main.go`

**Step 1: Add systray dependency**

```bash
cd cc-status-go && go get fyne.io/systray
```

**Step 2: Write tray implementation**

```go
// internal/tray/tray.go
package tray

import (
	"fmt"
	"strings"
	"path/filepath"

	"fyne.io/systray"
	"github.com/anthropics/cc-status-go/internal/session"
	"github.com/anthropics/cc-status-go/pkg/model"
)

type Tray struct {
	store        *session.Store
	sessionItems []*systray.MenuItem
	dismissItem  *systray.MenuItem
	quitItem     *systray.MenuItem
}

func NewTray(store *session.Store) *Tray {
	return &Tray{store: store}
}

func (t *Tray) OnReady() {
	systray.SetIcon(iconIdle)
	systray.SetTooltip("cc-status")

	t.dismissItem = systray.AddMenuItem("Dismiss All", "Dismiss all sessions")
	t.dismissItem.Hide()
	systray.AddSeparator()
	t.quitItem = systray.AddMenuItem("Quit", "Quit cc-status")

	t.store.SetOnChange(func() {
		t.refresh()
	})
	t.refresh()

	go func() {
		for {
			select {
			case <-t.dismissItem.ClickedCh:
				t.store.DismissAll()
			case <-t.quitItem.ClickedCh:
				systray.Quit()
			}
		}
	}()
}

func (t *Tray) OnExit() {}

func (t *Tray) refresh() {
	// Remove old session items
	for _, item := range t.sessionItems {
		item.Hide()
	}
	t.sessionItems = nil

	sorted := t.store.Sorted()

	// Update tray icon
	if len(sorted) == 0 {
		systray.SetIcon(iconIdle)
	} else {
		hasWaiting := false
		hasDone := false
		for _, s := range sorted {
			if s.Status == model.StatusWaiting {
				hasWaiting = true
			}
			if s.Status == model.StatusDone {
				hasDone = true
			}
		}
		if hasWaiting {
			systray.SetIcon(iconWaiting)
		} else if hasDone {
			systray.SetIcon(iconDone)
		} else {
			systray.SetIcon(iconActive)
		}
	}

	// systray doesn't support dynamic menu item insertion well,
	// so we pre-create items and show/hide them.
	// For simplicity, recreate menu on each refresh via SetTitle updates.
	// fyne-io/systray supports AddMenuItem at top via AddMenuItemBefore — but
	// the simplest approach: use SetTitle on pre-allocated items.

	// Actually, fyne-io/systray doesn't support removing items.
	// We need to pre-allocate a fixed number of slots.
	// For now, use tooltip + title approach.

	// Rebuild: we can only add items, not remove. So we allocate up to 20 slots,
	// show the ones we need, hide the rest.
	for len(t.sessionItems) < len(sorted) && len(t.sessionItems) < 20 {
		item := systray.AddMenuItemFirst("", "")
		t.sessionItems = append(t.sessionItems, item)
		idx := len(t.sessionItems) - 1
		go func(i int) {
			for range t.sessionItems[i].ClickedCh {
				sessions := t.store.Sorted()
				if i < len(sessions) {
					FocusTerminal(sessions[i].TerminalID)
				}
			}
		}(idx)
	}

	for i, item := range t.sessionItems {
		if i < len(sorted) {
			s := sorted[i]
			emoji := statusEmoji(s.Status)
			repo := filepath.Base(s.Cwd)
			title := fmt.Sprintf("%s %s", emoji, repo)
			if s.Branch != "" {
				title += fmt.Sprintf(" · %s", s.Branch)
			}
			if s.Summary != "" {
				summary := s.Summary
				if len(summary) > 50 {
					summary = summary[:50] + "…"
				}
				title += fmt.Sprintf(" — %s", summary)
			}
			item.SetTitle(title)
			item.Show()
		} else {
			item.Hide()
		}
	}

	if len(sorted) > 0 {
		t.dismissItem.Show()
	} else {
		t.dismissItem.Hide()
	}
}

func statusEmoji(s model.SessionStatus) string {
	switch s {
	case model.StatusWaiting:
		return "🟠"
	case model.StatusDone:
		return "🟢"
	case model.StatusActive:
		return "🔵"
	default:
		return "⚪"
	}
}
```

```go
// internal/tray/focus.go
package tray

import (
	"os/exec"
	"strings"
)

var knownTerminals = []string{
	"ghostty", "kitty", "alacritty", "wezterm",
	"gnome-terminal", "konsole", "xterm",
}

func FocusTerminal(terminalID *string) {
	if terminalID == nil {
		focusAnyTerminal()
		return
	}
	tid := *terminalID

	switch {
	case strings.HasPrefix(tid, "ghostty:"):
		launchApp("ghostty")
	case strings.HasPrefix(tid, "app:"):
		app := strings.TrimPrefix(tid, "app:")
		launchApp(strings.ToLower(app))
	case strings.HasPrefix(tid, "iterm:"):
		launchApp("iterm2")
	case strings.HasPrefix(tid, "warp:"):
		launchApp("warp")
	default:
		focusAnyTerminal()
	}
}

func focusAnyTerminal() {
	for _, name := range knownTerminals {
		if path, err := exec.LookPath(name); err == nil {
			exec.Command(path).Start()
			return
		}
	}
}

func launchApp(name string) {
	if path, err := exec.LookPath(name); err == nil {
		exec.Command(path).Start()
	} else {
		focusAnyTerminal()
	}
}
```

**Step 3: Write tray main**

```go
// cmd/cc-status-tray/main.go
package main

import (
	"log"
	"time"

	"fyne.io/systray"
	"github.com/anthropics/cc-status-go/internal/server"
	"github.com/anthropics/cc-status-go/internal/session"
	"github.com/anthropics/cc-status-go/internal/tray"
	"github.com/anthropics/cc-status-go/pkg/model"
)

func main() {
	store := session.NewStore()
	store.LoadFromDisk()

	srv := server.New(model.SocketPath(), func(e model.SessionEvent) {
		store.HandleEvent(e)
	})
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Periodic cleanup
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			store.CleanupStale()
		}
	}()

	t := tray.NewTray(store)
	systray.Run(t.OnReady, func() {
		srv.Stop()
		t.OnExit()
	})
}
```

**Step 4: Build tray binary**

```bash
cd cc-status-go && go build -o bin/cc-status-tray ./cmd/cc-status-tray
```

**Step 5: Commit**

```bash
git add cc-status-go/
git commit -m "feat(go): add Linux system tray UI with emoji status and terminal focus"
```

---

### Task 7: Integration Test + README

**Files:**
- Create: `cc-status-go/internal/integration_test.go`
- Create: `cc-status-go/README.md`

**Step 1: Write integration test (server + store + hook protocol)**

```go
// internal/integration_test.go
package internal

import (
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropics/cc-status-go/internal/server"
	"github.com/anthropics/cc-status-go/internal/session"
	"github.com/anthropics/cc-status-go/pkg/model"
)

func TestEndToEnd(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	store := session.NewStore()
	srv := server.New(sockPath, func(e model.SessionEvent) {
		store.HandleEvent(e)
	})
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()
	time.Sleep(50 * time.Millisecond)

	// Simulate hook sending events
	events := []model.SessionEvent{
		{SessionID: "s1", Event: model.StatusActive, Cwd: "/home/user/project-a", Branch: "main", Summary: "Working...", Timestamp: float64(time.Now().Unix())},
		{SessionID: "s2", Event: model.StatusWaiting, Cwd: "/home/user/project-b", Branch: "feat/x", Summary: "Waiting for input", Timestamp: float64(time.Now().Unix())},
	}

	for _, e := range events {
		conn, err := net.Dial("unix", sockPath)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(e)
		conn.Write(data)
		conn.Close()
	}

	time.Sleep(200 * time.Millisecond)

	all := store.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(all))
	}

	sorted := store.Sorted()
	// Waiting should come first
	if sorted[0].Status != model.StatusWaiting {
		t.Errorf("first sorted should be waiting, got %s", sorted[0].Status)
	}

	// Remove s1
	conn, _ := net.Dial("unix", sockPath)
	data, _ := json.Marshal(model.SessionEvent{SessionID: "s1", Event: model.StatusRemove, Cwd: "/tmp", Timestamp: float64(time.Now().Unix())})
	conn.Write(data)
	conn.Close()
	time.Sleep(200 * time.Millisecond)

	if len(store.All()) != 1 {
		t.Errorf("expected 1 session after remove, got %d", len(store.All()))
	}
}
```

**Step 2: Run all tests**

```bash
cd cc-status-go && go test ./... -v
```

**Step 3: Write README**

```markdown
# cc-status-go

Linux system tray app and cross-platform hook binary for monitoring Claude Code sessions.

## Build

```bash
go build -o bin/cc-status-tray ./cmd/cc-status-tray
go build -o bin/cc-status-hook ./cmd/cc-status-hook
```

## Install Hook

```bash
./bin/cc-status-hook install
```

This adds cc-status hooks to `~/.claude/settings.json`.

## Run

```bash
./bin/cc-status-tray
```

## Autostart (Linux)

Copy the desktop file to autostart:

```bash
cp cc-status-tray.desktop ~/.config/autostart/
```

## Uninstall Hook

```bash
./bin/cc-status-hook uninstall
```
```

**Step 4: Commit**

```bash
git add cc-status-go/
git commit -m "feat(go): add integration test and README"
```

---

### Task 8: Makefile + Desktop File

**Files:**
- Create: `cc-status-go/Makefile`
- Create: `cc-status-go/cc-status-tray.desktop`

**Step 1: Create Makefile**

```makefile
.PHONY: all build test clean install-hook

all: build

build: bin/cc-status-tray bin/cc-status-hook

bin/cc-status-tray: $(shell find . -name '*.go' -not -path './assets/*')
	go build -o $@ ./cmd/cc-status-tray

bin/cc-status-hook: $(shell find . -name '*.go' -not -path './assets/*')
	go build -o $@ ./cmd/cc-status-hook

test:
	go test ./... -v

clean:
	rm -rf bin/

install-hook: bin/cc-status-hook
	./bin/cc-status-hook install

install: build install-hook
	mkdir -p ~/.local/bin
	cp bin/cc-status-tray ~/.local/bin/
	cp bin/cc-status-hook ~/.local/bin/
	@echo "Installed to ~/.local/bin/"
```

**Step 2: Create desktop file**

```ini
[Desktop Entry]
Type=Application
Name=CC Status
Comment=Claude Code session monitor
Exec=cc-status-tray
Icon=cc-status
Terminal=false
Categories=Utility;
StartupNotify=false
```

**Step 3: Commit**

```bash
git add cc-status-go/
git commit -m "feat(go): add Makefile and desktop file for Linux install"
```

Plan complete and saved to `docs/plans/2026-03-09-go-linux-impl.md`. Two execution options:

**1. Subagent-Driven (this session)** — I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** — Open new session with executing-plans, batch execution with checkpoints

Which approach?