package tray

import (
	"os/exec"
	"strings"
)

var knownTerminals = []string{
	"ghostty", "kitty", "alacritty", "wezterm",
	"gnome-terminal", "konsole", "xterm",
}

// FocusTerminal attempts to raise the terminal identified by terminalID.
// On Linux there is no reliable cross-DE window activation API, so we
// simply launch the terminal binary (which typically focuses an existing
// instance or opens a new window).
func FocusTerminal(terminalID *string) {
	if terminalID == nil {
		focusAnyTerminal()
		return
	}
	tid := *terminalID

	switch {
	case strings.HasPrefix(tid, "ghostty:"):
		launchApp("ghostty")
	case strings.HasPrefix(tid, "app:"):
		app := strings.TrimPrefix(tid, "app:")
		launchApp(strings.ToLower(app))
	case strings.HasPrefix(tid, "iterm:"):
		launchApp("iterm2")
	case strings.HasPrefix(tid, "warp:"):
		launchApp("warp")
	default:
		focusAnyTerminal()
	}
}

func focusAnyTerminal() {
	for _, name := range knownTerminals {
		if path, err := exec.LookPath(name); err == nil {
			_ = exec.Command(path).Start()
			return
		}
	}
}

func launchApp(name string) {
	if path, err := exec.LookPath(name); err == nil {
		_ = exec.Command(path).Start()
	} else {
		focusAnyTerminal()
	}
}
