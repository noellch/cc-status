package session

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/anthropics/cc-status-go/internal/proc"
	"github.com/anthropics/cc-status-go/pkg/model"
)

// Store holds active sessions protected by a read-write mutex.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]model.SessionInfo
	onChange func()

	saveMu    sync.Mutex
	saveTimer *time.Timer
}

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{
		sessions: make(map[string]model.SessionInfo),
	}
}

// SetOnChange registers a callback invoked (without holding the lock) after mutations.
func (s *Store) SetOnChange(fn func()) {
	s.mu.Lock()
	s.onChange = fn
	s.mu.Unlock()
}

func (s *Store) notifyChange() {
	s.mu.RLock()
	fn := s.onChange
	s.mu.RUnlock()
	if fn != nil {
		fn()
	}
}

// HandleEvent processes a session event: upsert or remove.
func (s *Store) HandleEvent(e model.SessionEvent) {
	s.mu.Lock()
	if e.Event == model.StatusRemove {
		delete(s.sessions, e.SessionID)
	} else {
		info := model.SessionInfo{
			SessionID:    e.SessionID,
			Status:       e.Event,
			Cwd:          e.Cwd,
			Branch:       e.Branch,
			Summary:      e.Summary,
			TerminalID:   e.TerminalID,
			LastUpdated:  e.Timestamp,
			ParentPID:    e.ParentPID,
			PIDStartTime: e.PIDStartTime,
		}
		// Preserve PID info from earlier events if the new event doesn't include it.
		if info.ParentPID == 0 {
			if existing, ok := s.sessions[e.SessionID]; ok {
				info.ParentPID = existing.ParentPID
				info.PIDStartTime = existing.PIDStartTime
			}
		}
		s.sessions[e.SessionID] = info
	}
	s.mu.Unlock()
	s.notifyChange()
	s.scheduleSave()
}

// All returns a shallow copy of the sessions map.
func (s *Store) All() map[string]model.SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]model.SessionInfo, len(s.sessions))
	for k, v := range s.sessions {
		cp[k] = v
	}
	return cp
}

// statusPriority returns sort priority: waiting=0, done=1, active=2.
func statusPriority(status model.SessionStatus) int {
	switch status {
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

// Sorted returns sessions sorted by status priority (waiting > done > active),
// then by LastUpdated descending.
func (s *Store) Sorted() []model.SessionInfo {
	s.mu.RLock()
	result := make([]model.SessionInfo, 0, len(s.sessions))
	for _, v := range s.sessions {
		result = append(result, v)
	}
	s.mu.RUnlock()

	sort.Slice(result, func(i, j int) bool {
		pi, pj := statusPriority(result[i].Status), statusPriority(result[j].Status)
		if pi != pj {
			return pi < pj
		}
		return result[i].LastUpdated > result[j].LastUpdated
	})
	return result
}

// WaitingCount returns the number of sessions with status waiting.
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

// DoneCount returns the number of sessions with status done.
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

// DismissAll removes all sessions and fires onChange + scheduleSave.
func (s *Store) DismissAll() {
	s.mu.Lock()
	s.sessions = make(map[string]model.SessionInfo)
	s.mu.Unlock()
	s.notifyChange()
	s.scheduleSave()
}

// CleanupStale removes orphaned sessions:
// - waiting/done: no update for 30+ minutes
// - active: no update for 10+ minutes
// - any status: parent PID is dead (detected via PID + start time check)
func (s *Store) CleanupStale() {
	now := float64(time.Now().Unix())
	changed := false

	s.mu.Lock()
	for id, info := range s.sessions {
		// PID-based cleanup: if we have a parent PID recorded and
		// the process is no longer alive, remove immediately.
		if info.ParentPID > 0 && !proc.IsAlive(info.ParentPID, info.PIDStartTime) {
			delete(s.sessions, id)
			changed = true
			continue
		}

		// Time-based cleanup (fallback for sessions without PID info).
		age := now - info.LastUpdated
		switch info.Status {
		case model.StatusWaiting, model.StatusDone:
			if age > 30*60 {
				delete(s.sessions, id)
				changed = true
			}
		case model.StatusActive:
			if age > 10*60 {
				delete(s.sessions, id)
				changed = true
			}
		}
	}
	s.mu.Unlock()

	if changed {
		s.notifyChange()
		s.scheduleSave()
	}
}

// LoadFromDisk reads sessions from the JSON file and then runs CleanupStale.
func (s *Store) LoadFromDisk() {
	data, err := os.ReadFile(model.SessionsPath())
	if err != nil {
		return
	}
	var sessions map[string]model.SessionInfo
	if err := json.Unmarshal(data, &sessions); err != nil {
		return
	}
	s.mu.Lock()
	s.sessions = sessions
	s.mu.Unlock()
	s.CleanupStale()
}

// scheduleSave debounces disk writes by 1 second.
func (s *Store) scheduleSave() {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.saveTimer = time.AfterFunc(1*time.Second, s.saveToDisk)
}

// saveToDisk atomically writes sessions to disk.
func (s *Store) saveToDisk() {
	s.mu.RLock()
	data, err := json.Marshal(s.sessions)
	s.mu.RUnlock()
	if err != nil {
		return
	}

	dir := model.SocketDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}

	tmpPath := model.SessionsPath() + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmpPath, model.SessionsPath())
}

// FlushSave cancels pending debounce and saves immediately. Useful for testing.
func (s *Store) FlushSave() {
	s.saveMu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
	s.saveMu.Unlock()
	s.saveToDisk()
}
