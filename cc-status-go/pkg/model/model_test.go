package model

import (
	"encoding/json"
	"testing"
)

func TestSessionEventMarshalsToSnakeCaseKeys(t *testing.T) {
	tid := "term-1"
	ev := SessionEvent{
		SessionID:  "sess-abc",
		Event:      StatusActive,
		Cwd:        "/home/user/project",
		Branch:     "main",
		Summary:    "doing stuff",
		TerminalID: &tid,
		Timestamp:  1700000000.123,
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map error: %v", err)
	}

	expectedKeys := []string{"session_id", "event", "cwd", "branch", "summary", "terminal_id", "timestamp"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q not found in %s", key, string(data))
		}
	}

	// Ensure no camelCase keys leaked through
	unwantedKeys := []string{"sessionId", "SessionID", "terminalId", "TerminalID", "lastUpdated", "LastUpdated"}
	for _, key := range unwantedKeys {
		if _, ok := raw[key]; ok {
			t.Errorf("unexpected JSON key %q found in %s", key, string(data))
		}
	}
}

func TestSessionEventRoundTrip(t *testing.T) {
	tid := "term-42"
	original := SessionEvent{
		SessionID:  "sess-xyz",
		Event:      StatusWaiting,
		Cwd:        "/tmp/test",
		Branch:     "feature/cool",
		Summary:    "waiting for input",
		TerminalID: &tid,
		Timestamp:  1700000000.456,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded SessionEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.SessionID != original.SessionID {
		t.Errorf("SessionID: got %q, want %q", decoded.SessionID, original.SessionID)
	}
	if decoded.Event != original.Event {
		t.Errorf("Event: got %q, want %q", decoded.Event, original.Event)
	}
	if decoded.Cwd != original.Cwd {
		t.Errorf("Cwd: got %q, want %q", decoded.Cwd, original.Cwd)
	}
	if decoded.Branch != original.Branch {
		t.Errorf("Branch: got %q, want %q", decoded.Branch, original.Branch)
	}
	if decoded.Summary != original.Summary {
		t.Errorf("Summary: got %q, want %q", decoded.Summary, original.Summary)
	}
	if decoded.TerminalID == nil || *decoded.TerminalID != *original.TerminalID {
		t.Errorf("TerminalID: got %v, want %v", decoded.TerminalID, original.TerminalID)
	}
	if decoded.Timestamp != original.Timestamp {
		t.Errorf("Timestamp: got %v, want %v", decoded.Timestamp, original.Timestamp)
	}
}

func TestSessionInfoRoundTrip(t *testing.T) {
	tid := "term-99"
	original := SessionInfo{
		SessionID:   "sess-info-1",
		Status:      StatusDone,
		Cwd:         "/home/user",
		Branch:      "develop",
		Summary:     "all done",
		TerminalID:  &tid,
		LastUpdated: 1700000999.789,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded SessionInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.SessionID != original.SessionID {
		t.Errorf("SessionID mismatch")
	}
	if decoded.Status != original.Status {
		t.Errorf("Status: got %q, want %q", decoded.Status, original.Status)
	}
	if decoded.LastUpdated != original.LastUpdated {
		t.Errorf("LastUpdated: got %v, want %v", decoded.LastUpdated, original.LastUpdated)
	}
}

func TestNilTerminalIDProducesNull(t *testing.T) {
	ev := SessionEvent{
		SessionID:  "sess-nil",
		Event:      StatusDone,
		Cwd:        "/tmp",
		Branch:     "main",
		Summary:    "done",
		TerminalID: nil,
		Timestamp:  1700000000.0,
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	val, ok := raw["terminal_id"]
	if !ok {
		t.Fatal("terminal_id key missing from JSON")
	}
	if val != nil {
		t.Errorf("expected terminal_id to be null, got %v", val)
	}
}

func TestTimestampIsFloat64(t *testing.T) {
	ev := SessionEvent{
		SessionID: "sess-ts",
		Event:     StatusActive,
		Cwd:       "/tmp",
		Branch:    "main",
		Summary:   "test",
		Timestamp: 1700000000.123456,
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	ts, ok := raw["timestamp"]
	if !ok {
		t.Fatal("timestamp key missing")
	}

	// json.Unmarshal into interface{} produces float64 for numbers
	if _, ok := ts.(float64); !ok {
		t.Errorf("expected timestamp to be float64, got %T", ts)
	}

	if ts.(float64) != 1700000000.123456 {
		t.Errorf("timestamp value mismatch: got %v, want %v", ts, 1700000000.123456)
	}
}

func TestSocketPaths(t *testing.T) {
	dir := SocketDir()
	if dir == "" {
		t.Fatal("SocketDir() returned empty string")
	}

	sockPath := SocketPath()
	if sockPath == "" {
		t.Fatal("SocketPath() returned empty string")
	}

	sessPath := SessionsPath()
	if sessPath == "" {
		t.Fatal("SessionsPath() returned empty string")
	}
}
