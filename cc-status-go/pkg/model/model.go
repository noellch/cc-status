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
	SessionID    string        `json:"session_id"`
	Event        SessionStatus `json:"event"`
	Cwd          string        `json:"cwd"`
	Branch       string        `json:"branch"`
	Summary      string        `json:"summary"`
	TerminalID   *string       `json:"terminal_id"`
	Timestamp    float64       `json:"timestamp"`
	ParentPID    int           `json:"parent_pid,omitempty"`
	PIDStartTime string        `json:"pid_start_time,omitempty"`
}

type SessionInfo struct {
	SessionID    string        `json:"session_id"`
	Status       SessionStatus `json:"status"`
	Cwd          string        `json:"cwd"`
	Branch       string        `json:"branch"`
	Summary      string        `json:"summary"`
	TerminalID   *string       `json:"terminal_id"`
	LastUpdated  float64       `json:"last_updated"`
	ParentPID    int           `json:"parent_pid,omitempty"`
	PIDStartTime string        `json:"pid_start_time,omitempty"`
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
