package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var hookEvents = []string{
	"SessionStart",
	"UserPromptSubmit",
	"PreToolUse",
	"PostToolUse",
	"Stop",
	"Notification",
	"SessionEnd",
}

const commandMarker = "cc-status-hook"

// Install adds cc-status hook entries to ~/.claude/settings.json.
func Install() error {
	settingsPath := settingsFilePath()

	root, err := readOrCreateSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("reading settings: %w", err)
	}

	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	hookEntry, err := makeOurHookEntry()
	if err != nil {
		return fmt.Errorf("resolving hook path: %w", err)
	}

	var added []string

	for _, event := range hookEvents {
		matcherGroups, _ := hooks[event].([]any)

		if containsOurHook(matcherGroups) {
			continue
		}

		newGroup := map[string]any{
			"hooks": []any{hookEntry},
		}
		matcherGroups = append(matcherGroups, newGroup)
		hooks[event] = matcherGroups
		added = append(added, event)
	}

	root["hooks"] = hooks
	if err := writeSettings(root, settingsPath); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}

	if len(added) == 0 {
		fmt.Println("cc-status hooks already installed in ~/.claude/settings.json")
	} else {
		fmt.Printf("Installed cc-status hooks for events: %s\n", strings.Join(added, ", "))
		fmt.Printf("Settings written to %s\n", settingsPath)
	}
	return nil
}

// Uninstall removes cc-status hook entries from ~/.claude/settings.json.
func Uninstall() error {
	settingsPath := settingsFilePath()

	data, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		fmt.Println("No ~/.claude/settings.json found. Nothing to uninstall.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading settings: %w", err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		fmt.Println("Settings file is not valid JSON. Nothing to uninstall.")
		return nil
	}

	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		fmt.Println("No hooks section found. Nothing to uninstall.")
		return nil
	}

	var removed []string

	for event := range hooks {
		matcherGroups, ok := hooks[event].([]any)
		if !ok {
			continue
		}

		didRemove := false
		var filtered []any

		for _, groupRaw := range matcherGroups {
			group, ok := groupRaw.(map[string]any)
			if !ok {
				filtered = append(filtered, groupRaw)
				continue
			}

			groupHooks, ok := group["hooks"].([]any)
			if !ok {
				filtered = append(filtered, group)
				continue
			}

			before := len(groupHooks)
			var kept []any
			for _, h := range groupHooks {
				if !isOurHookEntry(h) {
					kept = append(kept, h)
				}
			}
			if len(kept) != before {
				didRemove = true
			}

			if len(kept) == 0 {
				// Entire matcher group only had our hook -- remove it
				continue
			}
			updated := make(map[string]any)
			for k, v := range group {
				updated[k] = v
			}
			updated["hooks"] = kept
			filtered = append(filtered, updated)
		}

		if didRemove {
			removed = append(removed, event)
		}

		if len(filtered) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = filtered
		}
	}

	root["hooks"] = hooks
	if err := writeSettings(root, settingsPath); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}

	if len(removed) == 0 {
		fmt.Println("No cc-status hooks found in ~/.claude/settings.json. Nothing to uninstall.")
	} else {
		fmt.Printf("Removed cc-status hooks from events: %s\n", strings.Join(removed, ", "))
		fmt.Printf("Settings written to %s\n", settingsPath)
	}
	return nil
}

// settingsFilePath returns ~/.claude/settings.json.
func settingsFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

// resolveHookPath returns the resolved path of the current executable.
func resolveHookPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

// makeOurHookEntry builds the hook entry dict.
func makeOurHookEntry() (map[string]any, error) {
	hookPath, err := resolveHookPath()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"type":    "command",
		"command": hookPath,
		"async":   true,
		"timeout": 5,
	}, nil
}

// readOrCreateSettings reads existing settings or creates a new file with {"hooks": {}}.
func readOrCreateSettings(path string) (map[string]any, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err == nil {
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err == nil {
			return obj, nil
		}
	}

	return map[string]any{
		"hooks": map[string]any{},
	}, nil
}

// writeSettings writes the settings dict as pretty-printed JSON,
// using flock for concurrent write safety and atomic write (tmp + rename).
func writeSettings(dict map[string]any, path string) error {
	data, err := json.MarshalIndent(dict, "", "  ")
	if err != nil {
		return err
	}
	// Append newline for POSIX compliance
	data = append(data, '\n')

	lockPath := path + ".lock"
	lockFD, err := syscall.Open(lockPath, syscall.O_CREAT|syscall.O_WRONLY, 0o644)
	if err == nil {
		defer func() {
			syscall.Flock(lockFD, syscall.LOCK_UN)
			syscall.Close(lockFD)
		}()
		_ = syscall.Flock(lockFD, syscall.LOCK_EX)
	}

	// Atomic write: write to tmp, then rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// containsOurHook checks if any matcher group already contains our hook entry.
func containsOurHook(matcherGroups []any) bool {
	for _, groupRaw := range matcherGroups {
		group, ok := groupRaw.(map[string]any)
		if !ok {
			continue
		}
		groupHooks, ok := group["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range groupHooks {
			if isOurHookEntry(h) {
				return true
			}
		}
	}
	return false
}

// isOurHookEntry checks if a hook entry is ours by looking for the command marker.
func isOurHookEntry(hookRaw any) bool {
	hook, ok := hookRaw.(map[string]any)
	if !ok {
		return false
	}
	command, ok := hook["command"].(string)
	if !ok {
		return false
	}
	return strings.Contains(command, commandMarker)
}
