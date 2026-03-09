package hook

// DetectTerminalIDFromEnv detects the terminal application from environment
// variables and returns a terminal identifier string, or nil if unknown.
// Takes a map (not os.Environ) for testability.
func DetectTerminalIDFromEnv(env map[string]string) *string {
	s := func(v string) *string { return &v }

	// 1. iTerm
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

	// 4. Ghostty
	if _, ok := env["GHOSTTY_BIN_DIR"]; ok {
		windowID := env["GHOSTTY_WINDOW_ID"]
		return s("ghostty:" + windowID)
	}
	if env["TERM_PROGRAM"] == "ghostty" {
		windowID := env["GHOSTTY_WINDOW_ID"]
		return s("ghostty:" + windowID)
	}

	// 5. Known TERM_PROGRAM values
	termProgramMap := map[string]string{
		"WezTerm":   "WezTerm",
		"zed":       "Zed",
		"Hyper":     "Hyper",
		"kitty":     "kitty",
		"Alacritty": "Alacritty",
		"cursor":    "Cursor",
		"vscode":    "Visual Studio Code",
	}
	if tp, ok := env["TERM_PROGRAM"]; ok {
		if appName, found := termProgramMap[tp]; found {
			return s("app:" + appName)
		}
	}

	// 6. TTY fallback
	if v, ok := env["TTY"]; ok {
		return s("terminal:" + v)
	}

	// 7. Any TERM_PROGRAM as final fallback
	if tp, ok := env["TERM_PROGRAM"]; ok {
		return s("app:" + tp)
	}

	// 8. Unknown
	return nil
}
