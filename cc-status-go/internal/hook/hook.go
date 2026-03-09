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

// hookInput represents the JSON structure received from Claude Code hooks via stdin.
type hookInput struct {
	HookEventName       string `json:"hook_event_name"`
	SessionID           string `json:"session_id"`
	Cwd                 string `json:"cwd"`
	LastAssistantMessage string `json:"last_assistant_message"`
	NotificationType    string `json:"notification_type"`
	Message             string `json:"message"`
}

// ParseHookInput parses the stdin JSON from Claude Code hooks and returns
// a SessionEvent, or nil if the input should be ignored.
func ParseHookInput(data []byte) *model.SessionEvent {
	if len(data) == 0 {
		return nil
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil
	}

	if input.HookEventName == "" {
		return nil
	}

	// Determine session ID with fallback
	sessionID := input.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("unknown-%d", os.Getpid())
	}

	// Determine cwd with fallback
	cwd := input.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	var status model.SessionStatus
	var summary string

	switch input.HookEventName {
	case "SessionStart":
		status = model.StatusActive
		summary = "Session started"

	case "UserPromptSubmit":
		status = model.StatusActive
		summary = "Working..."

	case "Stop":
		status = model.StatusWaiting
		msg := input.LastAssistantMessage
		if msg != "" {
			if len(msg) > 80 {
				summary = msg[:80] + "..."
			} else {
				summary = msg
			}
		} else {
			summary = "Waiting for input"
		}

	case "Notification":
		nt := input.NotificationType
		if nt == "permission_prompt" || nt == "idle_prompt" {
			status = model.StatusWaiting
			summary = input.Message
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

	// Detect terminal and branch
	env := envToMap()
	terminalID := DetectTerminalIDFromEnv(env)
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

// envToMap converts os.Environ() to a map for DetectTerminalIDFromEnv.
func envToMap() map[string]string {
	m := make(map[string]string)
	for _, entry := range os.Environ() {
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			m[entry[:idx]] = entry[idx+1:]
		}
	}
	return m
}

// GetCurrentBranch returns the current git branch for the given directory,
// or an empty string on error.
func GetCurrentBranch(cwd string) string {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// SendToSocket marshals the event and sends it to the Unix domain socket.
// All errors are silently ignored.
func SendToSocket(event *model.SessionEvent) {
	if event == nil {
		return
	}

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
