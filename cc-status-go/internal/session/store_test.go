package session

import (
	"testing"
	"time"

	"github.com/anthropics/cc-status-go/pkg/model"
)

func TestHandleEvent(t *testing.T) {
	s := NewStore()

	s.HandleEvent(model.SessionEvent{
		SessionID: "s1",
		Event:     model.StatusActive,
		Cwd:       "/tmp",
		Branch:    "main",
		Summary:   "doing stuff",
		Timestamp: float64(time.Now().Unix()),
	})

	all := s.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 session, got %d", len(all))
	}
	info, ok := all["s1"]
	if !ok {
		t.Fatal("session s1 not found")
	}
	if info.Status != model.StatusActive {
		t.Errorf("expected status active, got %s", info.Status)
	}
	if info.Cwd != "/tmp" {
		t.Errorf("expected cwd /tmp, got %s", info.Cwd)
	}
}

func TestHandleEventRemove(t *testing.T) {
	s := NewStore()

	now := float64(time.Now().Unix())
	s.HandleEvent(model.SessionEvent{
		SessionID: "s1",
		Event:     model.StatusActive,
		Cwd:       "/tmp",
		Timestamp: now,
	})
	s.HandleEvent(model.SessionEvent{
		SessionID: "s1",
		Event:     model.StatusRemove,
		Timestamp: now + 1,
	})

	all := s.All()
	if len(all) != 0 {
		t.Fatalf("expected 0 sessions after remove, got %d", len(all))
	}
}

func TestCleanupStale(t *testing.T) {
	s := NewStore()

	now := float64(time.Now().Unix())
	old := now - 31*60 // 31 minutes ago
	activeOld := now - 11*60 // 11 minutes ago

	// Old waiting session — should be removed
	s.HandleEvent(model.SessionEvent{
		SessionID: "old-waiting",
		Event:     model.StatusWaiting,
		Timestamp: old,
	})
	// Old done session — should be removed
	s.HandleEvent(model.SessionEvent{
		SessionID: "old-done",
		Event:     model.StatusDone,
		Timestamp: old,
	})
	// Old active session — should be removed (>10min)
	s.HandleEvent(model.SessionEvent{
		SessionID: "old-active",
		Event:     model.StatusActive,
		Timestamp: activeOld,
	})
	// Recent active session — should survive
	s.HandleEvent(model.SessionEvent{
		SessionID: "recent-active",
		Event:     model.StatusActive,
		Timestamp: now,
	})
	// Recent waiting session — should survive
	s.HandleEvent(model.SessionEvent{
		SessionID: "recent-waiting",
		Event:     model.StatusWaiting,
		Timestamp: now,
	})

	s.CleanupStale()

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 sessions after cleanup, got %d", len(all))
	}
	if _, ok := all["recent-active"]; !ok {
		t.Error("recent-active should survive cleanup")
	}
	if _, ok := all["recent-waiting"]; !ok {
		t.Error("recent-waiting should survive cleanup")
	}
}

func TestSortedSessions(t *testing.T) {
	s := NewStore()

	now := float64(time.Now().Unix())

	s.HandleEvent(model.SessionEvent{
		SessionID: "active1",
		Event:     model.StatusActive,
		Timestamp: now,
	})
	s.HandleEvent(model.SessionEvent{
		SessionID: "waiting1",
		Event:     model.StatusWaiting,
		Timestamp: now - 1,
	})
	s.HandleEvent(model.SessionEvent{
		SessionID: "done1",
		Event:     model.StatusDone,
		Timestamp: now - 2,
	})
	s.HandleEvent(model.SessionEvent{
		SessionID: "waiting2",
		Event:     model.StatusWaiting,
		Timestamp: now,
	})

	sorted := s.Sorted()
	if len(sorted) != 4 {
		t.Fatalf("expected 4 sessions, got %d", len(sorted))
	}

	// Expected order: waiting(0) > done(1) > active(2)
	// Within same status, by LastUpdated desc
	// waiting2 (now), waiting1 (now-1), done1 (now-2), active1 (now)
	if sorted[0].SessionID != "waiting2" {
		t.Errorf("expected waiting2 first, got %s", sorted[0].SessionID)
	}
	if sorted[1].SessionID != "waiting1" {
		t.Errorf("expected waiting1 second, got %s", sorted[1].SessionID)
	}
	if sorted[2].SessionID != "done1" {
		t.Errorf("expected done1 third, got %s", sorted[2].SessionID)
	}
	if sorted[3].SessionID != "active1" {
		t.Errorf("expected active1 fourth, got %s", sorted[3].SessionID)
	}
}

func TestDismissAll(t *testing.T) {
	s := NewStore()

	now := float64(time.Now().Unix())
	s.HandleEvent(model.SessionEvent{
		SessionID: "s1",
		Event:     model.StatusActive,
		Timestamp: now,
	})
	s.HandleEvent(model.SessionEvent{
		SessionID: "s2",
		Event:     model.StatusWaiting,
		Timestamp: now,
	})

	s.DismissAll()

	all := s.All()
	if len(all) != 0 {
		t.Fatalf("expected 0 sessions after dismiss, got %d", len(all))
	}
}

func TestWaitingAndDoneCount(t *testing.T) {
	s := NewStore()

	now := float64(time.Now().Unix())
	s.HandleEvent(model.SessionEvent{SessionID: "w1", Event: model.StatusWaiting, Timestamp: now})
	s.HandleEvent(model.SessionEvent{SessionID: "w2", Event: model.StatusWaiting, Timestamp: now})
	s.HandleEvent(model.SessionEvent{SessionID: "d1", Event: model.StatusDone, Timestamp: now})
	s.HandleEvent(model.SessionEvent{SessionID: "a1", Event: model.StatusActive, Timestamp: now})

	if s.WaitingCount() != 2 {
		t.Errorf("expected 2 waiting, got %d", s.WaitingCount())
	}
	if s.DoneCount() != 1 {
		t.Errorf("expected 1 done, got %d", s.DoneCount())
	}
}

func TestOnChangeCallback(t *testing.T) {
	s := NewStore()
	called := 0
	s.SetOnChange(func() { called++ })

	s.HandleEvent(model.SessionEvent{
		SessionID: "s1",
		Event:     model.StatusActive,
		Timestamp: float64(time.Now().Unix()),
	})

	if called != 1 {
		t.Errorf("expected onChange called 1 time, got %d", called)
	}

	s.DismissAll()
	if called != 2 {
		t.Errorf("expected onChange called 2 times, got %d", called)
	}
}
