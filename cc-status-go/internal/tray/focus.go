package tray

import (
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// FocusTerminal attempts to raise the terminal identified by terminalID.
// On macOS, uses "open -a" to activate apps (matches Swift's TerminalJumper).
// On Linux, falls back to launching the terminal binary.
func FocusTerminal(terminalID *string) {
	if terminalID == nil {
		focusAnyTerminal()
		return
	}
	tid := *terminalID

	switch {
	case strings.HasPrefix(tid, "ghostty:"):
		openApp("Ghostty")
	case strings.HasPrefix(tid, "iterm:"):
		openApp("iTerm")
	case strings.HasPrefix(tid, "warp:"):
		openApp("Warp")
	case strings.HasPrefix(tid, "app:"):
		appName := strings.TrimPrefix(tid, "app:")
		openApp(sanitizeAppName(appName))
	case strings.HasPrefix(tid, "terminal:"):
		openApp("Terminal")
	default:
		focusAnyTerminal()
	}
}

// sanitizeAppName strips characters that could break shell commands.
// Matches Swift's sanitizeAppName: only alphanumeric, spaces, hyphens, dots.
func sanitizeAppName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == ' ' || r == '-' || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// openApp activates an application by name.
// On macOS: uses "open -a <name>" (matches Swift's TerminalJumper.openApp).
// On Linux: tries to launch the binary directly.
func openApp(name string) {
	if runtime.GOOS == "darwin" {
		_ = exec.Command("/usr/bin/open", "-a", name).Start()
	} else {
		lower := strings.ToLower(name)
		if path, err := exec.LookPath(lower); err == nil {
			_ = exec.Command(path).Start()
		} else {
			focusAnyTerminal()
		}
	}
}

// knownTerminals lists terminals to try when we can't identify the specific one.
var knownTerminals []struct{ appName, binary string }

func init() {
	if runtime.GOOS == "darwin" {
		knownTerminals = []struct{ appName, binary string }{
			{"Ghostty", "ghostty"},
			{"iTerm", "iterm2"},
			{"Terminal", "terminal"},
			{"Warp", "warp"},
		}
	} else {
		knownTerminals = []struct{ appName, binary string }{
			{"Ghostty", "ghostty"},
			{"kitty", "kitty"},
			{"Alacritty", "alacritty"},
			{"WezTerm", "wezterm"},
			{"gnome-terminal", "gnome-terminal"},
			{"konsole", "konsole"},
			{"xterm", "xterm"},
		}
	}
}

// focusAnyTerminal tries to activate any known terminal.
// On macOS: uses "open -a" with a short timeout to avoid blocking (matches Swift's
// NSRunningApplication check — we use timeout instead since we can't query AppKit).
// On Linux: uses exec.LookPath to find an installed terminal.
func focusAnyTerminal() {
	if runtime.GOOS == "darwin" {
		for _, t := range knownTerminals {
			cmd := exec.Command("/usr/bin/open", "-a", t.appName)
			// Use Start + short wait instead of Run to avoid blocking on
			// "app not found" dialog. open -a exits quickly if app exists.
			if err := cmd.Start(); err != nil {
				continue
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				if err == nil {
					return // success
				}
			case <-time.After(500 * time.Millisecond):
				// Took too long — app probably doesn't exist, try next
				_ = cmd.Process.Kill()
			}
		}
	} else {
		for _, t := range knownTerminals {
			if path, err := exec.LookPath(t.binary); err == nil {
				_ = exec.Command(path).Start()
				return
			}
		}
	}
}
