package hook

import (
	"encoding/json"
	"testing"

	"github.com/anthropics/cc-status-go/pkg/model"
)

func TestParseHookInput_SessionStart(t *testing.T) {
	input := map[string]any{
		"hook_event_name": "SessionStart",
		"session_id":      "sess-123",
		"cwd":             "/tmp/project",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	if ev.Event != model.StatusActive {
		t.Errorf("expected status %q, got %q", model.StatusActive, ev.Event)
	}
	if ev.Summary != "Session started" {
		t.Errorf("expected summary %q, got %q", "Session started", ev.Summary)
	}
	if ev.SessionID != "sess-123" {
		t.Errorf("expected session_id %q, got %q", "sess-123", ev.SessionID)
	}
	if ev.Cwd != "/tmp/project" {
		t.Errorf("expected cwd %q, got %q", "/tmp/project", ev.Cwd)
	}
}

func TestParseHookInput_UserPromptSubmit(t *testing.T) {
	input := map[string]any{
		"hook_event_name": "UserPromptSubmit",
		"session_id":      "sess-456",
		"cwd":             "/home/user",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	if ev.Event != model.StatusActive {
		t.Errorf("expected status %q, got %q", model.StatusActive, ev.Event)
	}
	if ev.Summary != "Working..." {
		t.Errorf("expected summary %q, got %q", "Working...", ev.Summary)
	}
}

func TestParseHookInput_PreToolUse(t *testing.T) {
	input := map[string]any{
		"hook_event_name": "PreToolUse",
		"session_id":      "sess-pre",
		"cwd":             "/tmp",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	if ev.Event != model.StatusActive {
		t.Errorf("expected status %q, got %q", model.StatusActive, ev.Event)
	}
	if ev.Summary != "Working..." {
		t.Errorf("expected summary %q, got %q", "Working...", ev.Summary)
	}
}

func TestParseHookInput_PostToolUse(t *testing.T) {
	input := map[string]any{
		"hook_event_name": "PostToolUse",
		"session_id":      "sess-post",
		"cwd":             "/tmp",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	if ev.Event != model.StatusActive {
		t.Errorf("expected status %q, got %q", model.StatusActive, ev.Event)
	}
}

func TestParseHookInput_Stop_WithMessage(t *testing.T) {
	input := map[string]any{
		"hook_event_name":      "Stop",
		"session_id":           "sess-789",
		"cwd":                  "/tmp",
		"last_assistant_message": "I've completed the refactoring of the authentication module",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	if ev.Event != model.StatusWaiting {
		t.Errorf("expected status %q, got %q", model.StatusWaiting, ev.Event)
	}
	if ev.Summary != "I've completed the refactoring of the authentication module" {
		t.Errorf("unexpected summary: %q", ev.Summary)
	}
}

func TestParseHookInput_Stop_WithLongMessage(t *testing.T) {
	longMsg := "This is a very long message that exceeds eighty characters and should be truncated by the parser to keep the summary concise and readable"
	input := map[string]any{
		"hook_event_name":      "Stop",
		"session_id":           "sess-long",
		"cwd":                  "/tmp",
		"last_assistant_message": longMsg,
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	if len(ev.Summary) > 84 { // 80 + "..."
		t.Errorf("summary too long (%d chars): %q", len(ev.Summary), ev.Summary)
	}
	expected := longMsg[:80] + "..."
	if ev.Summary != expected {
		t.Errorf("expected summary %q, got %q", expected, ev.Summary)
	}
}

func TestParseHookInput_Stop_WithoutMessage(t *testing.T) {
	input := map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "sess-nope",
		"cwd":             "/tmp",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	if ev.Summary != "Waiting for input" {
		t.Errorf("expected summary %q, got %q", "Waiting for input", ev.Summary)
	}
}

func TestParseHookInput_Stop_EmptyMessage(t *testing.T) {
	input := map[string]any{
		"hook_event_name":      "Stop",
		"session_id":           "sess-empty",
		"cwd":                  "/tmp",
		"last_assistant_message": "",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	if ev.Summary != "Waiting for input" {
		t.Errorf("expected summary %q, got %q", "Waiting for input", ev.Summary)
	}
}

func TestParseHookInput_Notification_PermissionPrompt(t *testing.T) {
	input := map[string]any{
		"hook_event_name":  "Notification",
		"session_id":       "sess-notif",
		"cwd":              "/tmp",
		"notification_type": "permission_prompt",
		"message":          "Allow file write?",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	if ev.Event != model.StatusWaiting {
		t.Errorf("expected status %q, got %q", model.StatusWaiting, ev.Event)
	}
	if ev.Summary != "Allow file write?" {
		t.Errorf("expected summary %q, got %q", "Allow file write?", ev.Summary)
	}
}

func TestParseHookInput_Notification_IdlePrompt(t *testing.T) {
	input := map[string]any{
		"hook_event_name":  "Notification",
		"session_id":       "sess-idle",
		"cwd":              "/tmp",
		"notification_type": "idle_prompt",
		"message":          "Session idle",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	if ev.Event != model.StatusWaiting {
		t.Errorf("expected status %q, got %q", model.StatusWaiting, ev.Event)
	}
	if ev.Summary != "Session idle" {
		t.Errorf("expected summary %q, got %q", "Session idle", ev.Summary)
	}
}

func TestParseHookInput_Notification_OtherType(t *testing.T) {
	input := map[string]any{
		"hook_event_name":  "Notification",
		"session_id":       "sess-other",
		"cwd":              "/tmp",
		"notification_type": "info",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev != nil {
		t.Errorf("expected nil for other notification type, got %+v", ev)
	}
}

func TestParseHookInput_Notification_NoMessage(t *testing.T) {
	input := map[string]any{
		"hook_event_name":  "Notification",
		"session_id":       "sess-nomsg",
		"cwd":              "/tmp",
		"notification_type": "permission_prompt",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	if ev.Summary != "Needs attention" {
		t.Errorf("expected summary %q, got %q", "Needs attention", ev.Summary)
	}
}

func TestParseHookInput_SessionEnd(t *testing.T) {
	input := map[string]any{
		"hook_event_name": "SessionEnd",
		"session_id":      "sess-end",
		"cwd":             "/tmp",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	if ev.Event != model.StatusRemove {
		t.Errorf("expected status %q, got %q", model.StatusRemove, ev.Event)
	}
	if ev.Summary != "" {
		t.Errorf("expected empty summary, got %q", ev.Summary)
	}
}

func TestParseHookInput_Unknown(t *testing.T) {
	input := map[string]any{
		"hook_event_name": "SomethingElse",
		"session_id":      "sess-unk",
		"cwd":             "/tmp",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev != nil {
		t.Errorf("expected nil for unknown event, got %+v", ev)
	}
}

func TestParseHookInput_Empty(t *testing.T) {
	ev := ParseHookInput(nil)
	if ev != nil {
		t.Errorf("expected nil for nil input, got %+v", ev)
	}

	ev = ParseHookInput([]byte{})
	if ev != nil {
		t.Errorf("expected nil for empty input, got %+v", ev)
	}
}

func TestParseHookInput_InvalidJSON(t *testing.T) {
	ev := ParseHookInput([]byte("not json"))
	if ev != nil {
		t.Errorf("expected nil for invalid JSON, got %+v", ev)
	}
}

func TestParseHookInput_FallbackSessionID(t *testing.T) {
	input := map[string]any{
		"hook_event_name": "SessionStart",
		"cwd":             "/tmp",
	}
	data, _ := json.Marshal(input)

	ev := ParseHookInput(data)
	if ev == nil {
		t.Fatal("expected non-nil event")
	}
	// Should have a fallback session ID starting with "unknown-"
	if len(ev.SessionID) < 8 || ev.SessionID[:8] != "unknown-" {
		t.Errorf("expected fallback session_id starting with 'unknown-', got %q", ev.SessionID)
	}
}

// --- Terminal detection tests ---

func TestDetectTerminalIDFromEnv_iTerm(t *testing.T) {
	env := map[string]string{
		"ITERM_SESSION_ID": "w0t0p0:ABC-123",
	}
	tid := DetectTerminalIDFromEnv(env)
	if tid == nil {
		t.Fatal("expected non-nil terminal ID")
	}
	if *tid != "iterm:w0t0p0:ABC-123" {
		t.Errorf("expected %q, got %q", "iterm:w0t0p0:ABC-123", *tid)
	}
}

func TestDetectTerminalIDFromEnv_Ghostty(t *testing.T) {
	env := map[string]string{
		"GHOSTTY_BIN_DIR":  "/usr/local/bin",
		"GHOSTTY_WINDOW_ID": "42",
	}
	tid := DetectTerminalIDFromEnv(env)
	if tid == nil {
		t.Fatal("expected non-nil terminal ID")
	}
	if *tid != "ghostty:42" {
		t.Errorf("expected %q, got %q", "ghostty:42", *tid)
	}
}

func TestDetectTerminalIDFromEnv_GhosttyByTermProgram(t *testing.T) {
	env := map[string]string{
		"TERM_PROGRAM":     "ghostty",
		"GHOSTTY_WINDOW_ID": "99",
	}
	tid := DetectTerminalIDFromEnv(env)
	if tid == nil {
		t.Fatal("expected non-nil terminal ID")
	}
	if *tid != "ghostty:99" {
		t.Errorf("expected %q, got %q", "ghostty:99", *tid)
	}
}

func TestDetectTerminalIDFromEnv_TERM_PROGRAM_WezTerm(t *testing.T) {
	env := map[string]string{
		"TERM_PROGRAM": "WezTerm",
	}
	tid := DetectTerminalIDFromEnv(env)
	if tid == nil {
		t.Fatal("expected non-nil terminal ID")
	}
	if *tid != "app:WezTerm" {
		t.Errorf("expected %q, got %q", "app:WezTerm", *tid)
	}
}

func TestDetectTerminalIDFromEnv_TERM_PROGRAM_Cursor(t *testing.T) {
	env := map[string]string{
		"TERM_PROGRAM": "cursor",
	}
	tid := DetectTerminalIDFromEnv(env)
	if tid == nil {
		t.Fatal("expected non-nil terminal ID")
	}
	if *tid != "app:Cursor" {
		t.Errorf("expected %q, got %q", "app:Cursor", *tid)
	}
}

func TestDetectTerminalIDFromEnv_TERM_PROGRAM_VSCode(t *testing.T) {
	env := map[string]string{
		"TERM_PROGRAM": "vscode",
	}
	tid := DetectTerminalIDFromEnv(env)
	if tid == nil {
		t.Fatal("expected non-nil terminal ID")
	}
	if *tid != "app:Visual Studio Code" {
		t.Errorf("expected %q, got %q", "app:Visual Studio Code", *tid)
	}
}

func TestDetectTerminalIDFromEnv_TTY(t *testing.T) {
	env := map[string]string{
		"TTY": "/dev/ttys003",
	}
	tid := DetectTerminalIDFromEnv(env)
	if tid == nil {
		t.Fatal("expected non-nil terminal ID")
	}
	if *tid != "terminal:/dev/ttys003" {
		t.Errorf("expected %q, got %q", "terminal:/dev/ttys003", *tid)
	}
}

func TestDetectTerminalIDFromEnv_FallbackTermProgram(t *testing.T) {
	env := map[string]string{
		"TERM_PROGRAM": "SomeUnknownTerminal",
	}
	tid := DetectTerminalIDFromEnv(env)
	if tid == nil {
		t.Fatal("expected non-nil terminal ID")
	}
	if *tid != "app:SomeUnknownTerminal" {
		t.Errorf("expected %q, got %q", "app:SomeUnknownTerminal", *tid)
	}
}

func TestDetectTerminalIDFromEnv_Empty(t *testing.T) {
	env := map[string]string{}
	tid := DetectTerminalIDFromEnv(env)
	if tid != nil {
		t.Errorf("expected nil for empty env, got %q", *tid)
	}
}

func TestDetectTerminalIDFromEnv_WarpSession(t *testing.T) {
	env := map[string]string{
		"WARP_SESSION_ID": "warp-abc-123",
	}
	tid := DetectTerminalIDFromEnv(env)
	if tid == nil {
		t.Fatal("expected non-nil terminal ID")
	}
	if *tid != "warp:warp-abc-123" {
		t.Errorf("expected %q, got %q", "warp:warp-abc-123", *tid)
	}
}

func TestDetectTerminalIDFromEnv_TermSessionID(t *testing.T) {
	env := map[string]string{
		"TERM_SESSION_ID": "session-xyz",
	}
	tid := DetectTerminalIDFromEnv(env)
	if tid == nil {
		t.Fatal("expected non-nil terminal ID")
	}
	if *tid != "terminal:session-xyz" {
		t.Errorf("expected %q, got %q", "terminal:session-xyz", *tid)
	}
}
