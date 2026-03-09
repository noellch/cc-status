# CC-Status

A macOS menu bar app that gives you real-time visibility into all your [Claude Code](https://docs.anthropic.com/en/docs/claude-code) sessions.

When you're running multiple Claude Code sessions across different terminals and projects, it's easy to lose track of which ones need your attention. CC-Status hooks into Claude Code's event system and shows you a live summary right in the menu bar — so you always know when a session is waiting for approval, has finished a task, or is still working.

## How It Works

```
Claude Code Session
        │
        ▼
  cc-status-hook          ← Claude Code hook (reads event from stdin)
        │
        ▼
  Unix Domain Socket      ← ~/.cc-status/cc-status.sock
        │
        ▼
  CCStatus Menu Bar App   ← Updates icon, shows session list
```

1. **Claude Code** fires hook events (session start, stop, notifications, etc.)
2. **cc-status-hook** receives the event via stdin JSON, detects which terminal/IDE it came from, and forwards it over a Unix socket
3. **CCStatus app** updates the menu bar icon with colored status dots and maintains a dropdown list of all sessions

## Menu Bar

The menu bar icon shows colored dots representing your sessions:

| Color | Status | Meaning |
|-------|--------|---------|
| Amber | Waiting | Needs your input — permission prompt, approval, or idle |
| Green | Done | Task completed |
| Blue | Active | Currently working |

Click any session in the dropdown to jump directly to its terminal window or IDE.

## Supported Terminals & IDEs

CC-Status can detect and jump to sessions running in:

- **iTerm2** — jumps to the exact tab/session
- **Terminal.app** — jumps to the exact tab
- **Ghostty**, **Warp** — opens the app
- **VS Code**, **Cursor**, **Zed**, **Windsurf** — opens the editor

## Installation

### Prerequisites

- macOS 13+
- Swift 6.0+ toolchain
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) installed

### Build & Install

```bash
make install
```

This will:
1. Build both the menu bar app and the hook binary in release mode
2. Bundle `CCStatus.app` and copy it to `~/Applications/`
3. Install `cc-status-hook` to `~/.local/bin/`
4. Register hooks in `~/.claude/settings.json` for all relevant Claude Code events

### Uninstall

```bash
make uninstall
```

Removes the app, hook binary, hook registrations, and `~/.cc-status/` config directory.

## Project Structure

```
Sources/
├── CCStatus/                  # Menu bar app
│   ├── CCStatus.swift         # Entry point
│   ├── AppDelegate.swift      # App lifecycle, session restoration
│   ├── StatusBarController.swift  # Menu bar UI & interactions
│   ├── SessionStore.swift     # Session state management & persistence
│   ├── SocketServer.swift     # Unix domain socket listener
│   └── TerminalJumper.swift   # Terminal/IDE focus & navigation
├── CCStatusHook/              # Hook CLI binary
│   └── main.swift             # Event processing & socket client
└── CCStatusShared/            # Shared library
    ├── Models.swift           # SessionStatus, SessionEvent, CCStatusConfig
    ├── SocketAddress.swift    # Unix socket address helpers
    └── HookInstaller.swift    # Hook registration in Claude settings
```

## Design Decisions

- **Unix domain socket** over HTTP — no ports to configure, no network exposure, file-permission-based access control
- **Async hooks** — hooks run with `async: true` and a 5-second timeout, so they never block Claude Code
- **No external dependencies** — built entirely on Foundation, AppKit, and Darwin APIs
- **Graceful degradation** — if the menu bar app isn't running, the hook silently exits without error
- **Auto-cleanup** — stale sessions are automatically removed (30 min for waiting/done, 10 min for orphaned active sessions)
- **Persistent state** — sessions survive app restarts via `~/.cc-status/sessions.json`

## License

MIT
