package server

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/cc-status-go/pkg/model"
)

func TestServerReceivesEvent(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	var mu sync.Mutex
	var received *model.SessionEvent

	srv := New(sockPath, func(e model.SessionEvent) {
		mu.Lock()
		received = &e
		mu.Unlock()
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer srv.Stop()

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	event := model.SessionEvent{
		SessionID: "sess-123",
		Event:     model.StatusActive,
		Cwd:       "/tmp",
		Branch:    "main",
		Summary:   "doing stuff",
		Timestamp: 1234567890.0,
	}
	data, _ := json.Marshal(event)
	_, err = conn.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	conn.Close()

	// Wait for the event to be processed
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := received
		mu.Unlock()
		if got != nil {
			if got.SessionID != "sess-123" {
				t.Errorf("expected session_id sess-123, got %s", got.SessionID)
			}
			if got.Event != model.StatusActive {
				t.Errorf("expected event active, got %s", got.Event)
			}
			if got.Cwd != "/tmp" {
				t.Errorf("expected cwd /tmp, got %s", got.Cwd)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for event")
}

func TestServerRejectsOversizedMessage(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	var mu sync.Mutex
	eventReceived := false

	srv := New(sockPath, func(e model.SessionEvent) {
		mu.Lock()
		eventReceived = true
		mu.Unlock()
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer srv.Stop()

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	// Send 128KB of data — exceeds the 64KB limit
	bigData := strings.Repeat("x", 128*1024)
	_, _ = conn.Write([]byte(bigData))
	conn.Close()

	// Give the server time to process
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	got := eventReceived
	mu.Unlock()
	if got {
		t.Error("expected no event for oversized message, but one was received")
	}
}

func TestServerStaleSocketCleanup(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	// Create a fake stale socket file
	if err := os.WriteFile(sockPath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("failed to create fake socket file: %v", err)
	}

	var mu sync.Mutex
	var received *model.SessionEvent

	srv := New(sockPath, func(e model.SessionEvent) {
		mu.Lock()
		received = &e
		mu.Unlock()
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("Start failed (should have cleaned stale socket): %v", err)
	}
	defer srv.Stop()

	// Verify we can connect and send events
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial failed after stale cleanup: %v", err)
	}

	event := model.SessionEvent{
		SessionID: "sess-456",
		Event:     model.StatusWaiting,
		Timestamp: 1234567890.0,
	}
	data, _ := json.Marshal(event)
	_, _ = conn.Write(data)
	conn.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := received
		mu.Unlock()
		if got != nil {
			if got.SessionID != "sess-456" {
				t.Errorf("expected session_id sess-456, got %s", got.SessionID)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for event after stale socket cleanup")
}
