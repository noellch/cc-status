package hook

// DetectTerminalIDFromEnv detects the terminal application from environment
// variables and returns a terminal identifier string, or nil if unknown.
// Takes a map (not os.Environ) for testability.
// Detection order matches Swift's detectTerminalId().
func DetectTerminalIDFromEnv(env map[string]string) *string {
	s := func(v string) *string { return &v }

	// 1. iTerm (most specific — has session ID)
	if v, ok := env["ITERM_SESSION_ID"]; ok {
		return s("iterm:" + v)
	}

	// 2. macOS Terminal.app (TERM_SESSION_ID)
	if v, ok := env["TERM_SESSION_ID"]; ok {
		return s("terminal:" + v)
	}

	// 3. Warp
	if v, ok := env["WARP_SESSION_ID"]; ok {
		return s("warp:" + v)
	}

	// 4. Ghostty (check env vars before TERM_PROGRAM)
	if _, ok := env["GHOSTTY_BIN_DIR"]; ok {
		windowID := env["GHOSTTY_WINDOW_ID"]
		return s("ghostty:" + windowID)
	}
	if env["TERM_PROGRAM"] == "ghostty" {
		windowID := env["GHOSTTY_WINDOW_ID"]
		return s("ghostty:" + windowID)
	}

	// 5. __CFBundleIdentifier (macOS — most reliable app detection)
	// Matches Swift's bundleToApp lookup.
	bundleToApp := map[string]string{
		"com.todesktop.230313mzl4w4u92": "Cursor",
		"com.microsoft.VSCode":           "Visual Studio Code",
		"com.microsoft.VSCodeInsiders":   "Visual Studio Code - Insiders",
		"dev.zed.Zed":                    "Zed",
		"com.github.wez.wezterm":         "WezTerm",
		"net.kovidgoyal.kitty":           "kitty",
		"io.alacritty":                   "Alacritty",
		"co.zeit.hyper":                  "Hyper",
	}
	if bundleID, ok := env["__CFBundleIdentifier"]; ok {
		if appName, found := bundleToApp[bundleID]; found {
			return s("app:" + appName)
		}
	}

	// 6. IDE-specific env vars (Cursor vs VS Code disambiguation)
	// Matches Swift's CURSOR_TRACE_ID / VSCODE_PID logic.
	if _, ok := env["CURSOR_TRACE_ID"]; ok {
		return s("app:Cursor")
	}
	if env["TERM_PROGRAM"] == "cursor" {
		return s("app:Cursor")
	}
	if _, ok := env["VSCODE_PID"]; ok {
		// VSCODE_PID is set by both VS Code and Cursor.
		// If CURSOR_TRACE_ID was set, we'd have returned above.
		return s("app:Visual Studio Code")
	}
	if env["TERM_PROGRAM"] == "vscode" {
		return s("app:Visual Studio Code")
	}

	// 7. Known TERM_PROGRAM values
	termProgramMap := map[string]string{
		"WezTerm":   "WezTerm",
		"zed":       "Zed",
		"Hyper":     "Hyper",
		"kitty":     "kitty",
		"Alacritty": "Alacritty",
	}
	if tp, ok := env["TERM_PROGRAM"]; ok {
		if appName, found := termProgramMap[tp]; found {
			return s("app:" + appName)
		}
	}

	// 8. tmux — use TTY if available, otherwise nil (matches Swift)
	if env["TERM_PROGRAM"] == "tmux" {
		if tty, ok := env["TTY"]; ok {
			return s("terminal:" + tty)
		}
		return nil
	}

	// 9. TTY fallback
	if v, ok := env["TTY"]; ok {
		return s("terminal:" + v)
	}

	// 10. Any TERM_PROGRAM as final fallback
	if tp, ok := env["TERM_PROGRAM"]; ok {
		return s("app:" + tp)
	}

	// 11. Unknown
	return nil
}
