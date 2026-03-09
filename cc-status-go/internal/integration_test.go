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
	sendEvent := func(e model.SessionEvent) {
		conn, err := net.Dial("unix", sockPath)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(e)
		conn.Write(data)
		conn.Close()
	}

	now := float64(time.Now().Unix())

	// Send two events
	sendEvent(model.SessionEvent{SessionID: "s1", Event: model.StatusActive, Cwd: "/home/user/project-a", Branch: "main", Summary: "Working...", Timestamp: now})
	sendEvent(model.SessionEvent{SessionID: "s2", Event: model.StatusWaiting, Cwd: "/home/user/project-b", Branch: "feat/x", Summary: "Waiting for input", Timestamp: now})

	time.Sleep(200 * time.Millisecond)

	// Verify both sessions exist
	all := store.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(all))
	}

	// Verify sorting (waiting first)
	sorted := store.Sorted()
	if sorted[0].Status != model.StatusWaiting {
		t.Errorf("first sorted should be waiting, got %s", sorted[0].Status)
	}
	if sorted[1].Status != model.StatusActive {
		t.Errorf("second sorted should be active, got %s", sorted[1].Status)
	}

	// Remove s1
	sendEvent(model.SessionEvent{SessionID: "s1", Event: model.StatusRemove, Cwd: "/tmp", Timestamp: now})
	time.Sleep(200 * time.Millisecond)

	all = store.All()
	if len(all) != 1 {
		t.Errorf("expected 1 session after remove, got %d", len(all))
	}
	if _, ok := all["s2"]; !ok {
		t.Error("expected s2 to remain")
	}

	// DismissAll
	store.DismissAll()
	if len(store.All()) != 0 {
		t.Error("expected 0 sessions after dismiss all")
	}
}
