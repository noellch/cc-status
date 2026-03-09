# Plan: CC Status — Complete Implementation

## Task 1: Rewrite Hook Script to Read Stdin JSON
- **Files:** `Sources/CCStatusHook/main.swift`
- **Approach:** Remove CLI arg-based event type detection. Read all context from stdin JSON. Parse `hook_event_name` to determine event type. Map events:
  - `SessionStart` → active, summary: "Session started"
  - `UserPromptSubmit` → active, summary: "Working..."
  - `Stop` → waiting, summary: truncate `last_assistant_message` to ~80 chars
  - `Notification` (permission_prompt/idle_prompt) → waiting, summary: `message` field
  - `SessionEnd` → new "remove" event type
- **Depends on:** Task 2 (needs `remove` event in shared models)
- **Acceptance criteria:** Hook script reads stdin JSON, extracts session_id/cwd/hook_event_name, sends correct SessionEvent to socket. Works when invoked as `cc-status-hook` with no args.

## Task 2: Add SessionEnd / Remove Support to Shared Models
- **Files:** `Sources/CCStatusShared/Models.swift`, `Sources/CCStatus/SessionStore.swift`
- **Approach:** Add `remove` case to `SessionStatus` enum. In `SessionStore.handleEvent`, when status is `.remove`, delete the session from the dictionary instead of updating it.
- **Depends on:** none
- **Acceptance criteria:** Sending a `remove` event removes the session from the store and updates the menu bar icon/badge.

## Task 3: Session Auto-Cleanup Timer
- **Files:** `Sources/CCStatus/SessionStore.swift`, `Sources/CCStatus/AppDelegate.swift`
- **Approach:** Add a Timer that runs every 60 seconds. Remove sessions that have been in WAITING or DONE state for more than 30 minutes without any update (stale sessions where CC exited without firing SessionEnd). Configurable timeout.
- **Depends on:** none
- **Acceptance criteria:** Stale sessions are automatically removed. Active sessions are never auto-removed.

## Task 4: Install-Hooks CLI Command
- **Files:** `Sources/CCStatusHook/main.swift` (add subcommand), `Sources/CCStatusShared/HookInstaller.swift` (new)
- **Approach:** When invoked as `cc-status-hook install`, read `~/.claude/settings.json`, merge our hook config into existing hooks without overwriting user's other hooks. Our hook config:
  ```json
  {
    "SessionStart": [{"hooks": [{"type": "command", "command": "cc-status-hook", "async": true, "timeout": 5}]}],
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "cc-status-hook", "async": true, "timeout": 5}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "cc-status-hook", "async": true, "timeout": 5}]}],
    "Notification": [{"hooks": [{"type": "command", "command": "cc-status-hook", "async": true, "timeout": 5}]}],
    "SessionEnd": [{"hooks": [{"type": "command", "command": "cc-status-hook", "async": true, "timeout": 5}]}]
  }
  ```
  Also support `cc-status-hook uninstall` to remove our hooks.
- **Depends on:** none
- **Acceptance criteria:** Running `cc-status-hook install` adds hooks to settings.json without destroying existing config. `uninstall` cleanly removes them. Idempotent (running install twice doesn't duplicate).

## Task 5: App Bundle Build Script (Makefile)
- **Files:** `Makefile` (new)
- **Approach:** Create a Makefile with targets:
  - `build` — swift build -c release
  - `bundle` — create CCStatus.app bundle with Info.plist, copy binary
  - `install` — copy CCStatus.app to /Applications, copy cc-status-hook to /usr/local/bin, run install-hooks
  - `uninstall` — reverse of install
- **Depends on:** Task 4 (install target calls install-hooks)
- **Acceptance criteria:** `make install` produces a working setup from scratch. `make uninstall` cleanly removes everything.

## Task 6: Launch at Login Support
- **Files:** `Sources/CCStatus/AppDelegate.swift`, `Sources/CCStatus/StatusBarController.swift`
- **Approach:** Use `SMAppService.mainApp` (macOS 13+) to register/unregister as login item. Add "Launch at Login" toggle in the dropdown menu's Settings section.
- **Depends on:** none
- **Acceptance criteria:** Toggle in menu enables/disables launch at login. State persists across app restarts.

## Execution Order

```
Task 2 (shared models) ──┐
Task 3 (auto-cleanup)    ├──▶ Task 1 (hook script rewrite)
Task 4 (install-hooks)   │
Task 5 (Makefile) ───────┘──▶ depends on Task 4
Task 6 (launch at login) ────▶ independent
```

Independent tasks (2, 3, 4, 6) can be parallelized.
Task 1 depends on Task 2.
Task 5 depends on Task 4.
