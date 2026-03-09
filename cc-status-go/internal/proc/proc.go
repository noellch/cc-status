// Package proc provides cross-platform process liveness checks.
package proc

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// IsAlive checks if a process with the given PID is still running
// AND was started at startTime (to guard against PID reuse).
// startTime is the process start time as reported by GetStartTime.
// If startTime is empty, only the PID existence check is performed.
func IsAlive(pid int, startTime string) bool {
	if pid <= 0 {
		return false
	}

	// First check: is the PID alive at all?
	// Signal 0 doesn't send a signal but checks if the process exists.
	err := syscall.Kill(pid, 0)
	if err != nil {
		return false // process doesn't exist
	}

	// If no start time to verify, PID exists = alive.
	if startTime == "" {
		return true
	}

	// Second check: verify start time matches (guards against PID reuse).
	currentStart := GetStartTime(pid)
	if currentStart == "" {
		// Can't determine start time — fall back to PID-only check.
		return true
	}

	return currentStart == startTime
}

// GetStartTime returns a string identifying when the process with the given PID
// was started. Returns "" if the process doesn't exist or the info can't be read.
// The returned string is opaque — only useful for equality comparison.
func GetStartTime(pid int) string {
	// Use "ps -p <pid> -o lstart=" which works on both macOS and Linux.
	// lstart gives a full timestamp like "Mon Mar 10 12:34:56 2025".
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "lstart=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
