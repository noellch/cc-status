# Design: cc-status Go/Linux Port

## Decision Summary

- **Strategy**: Keep Swift for macOS (native UI quality), add Go for Linux
- **Hook binary**: Rewrite in Go (single static binary, no Swift toolchain needed on Linux)
- **Tray library**: fyne-io/systray (most mature, supports Linux AppIndicator/SNI)
- **Colors**: Emoji prefixes in menu items (🟠🟢🔵), tray icon swaps colored PNGs by state
- **Click action**: Focus terminal app (not specific tab), using `xdg-open` or `wmctrl`

## Architecture

```
cc-status/                    (existing Swift, macOS only — unchanged)
cc-status-go/                 (new Go module)
├── cmd/
│   ├── cc-status-tray/       Linux tray app (main binary)
│   │   └── main.go
│   └── cc-status-hook/       Cross-platform hook binary
│       └── main.go
├── internal/
│   ├── hook/                 Hook stdin parsing + socket send
│   │   ├── hook.go
│   │   └── terminal.go       Terminal ID detection
│   ├── server/               Unix socket server
│   │   └── server.go
│   ├── session/              Session store + cleanup
│   │   └── store.go
│   └── tray/                 System tray UI (Linux)
│       ├── tray.go
│       ├── menu.go
│       └── icons.go          Embedded PNG icons
├── pkg/
│   └── model/                Shared models (SessionEvent, SessionStatus, config)
│       └── model.go
├── assets/
│   ├── idle.png              Gray dot (no sessions)
│   ├── active.png            Blue dot
│   ├── waiting.png           Orange dot
│   └── done.png              Green dot
├── go.mod
└── go.sum
```

## Shared Protocol

The Go hook binary speaks the exact same Unix socket + JSON protocol as the Swift version. This means:

- The Go hook works with both the Swift macOS app and the Go Linux tray app
- Users can mix and match (e.g., Go hook on macOS sending to Swift tray)
- Socket path: `~/.cc-status/cc-status.sock`
- JSON format: snake_case keys, timestamps as seconds-since-epoch

```json
{
  "session_id": "abc123",
  "event": "waiting",
  "cwd": "/home/user/project",
  "branch": "main",
  "summary": "Waiting for input",
  "terminal_id": "ghostty:window-1",
  "timestamp": 1741520400.0
}
```

## Component Details

### 1. Hook Binary (`cmd/cc-status-hook`)

Reads Claude Code hook stdin JSON, maps to SessionEvent, sends to socket.

Responsibilities:
- Parse stdin JSON (`hook_event_name`, `session_id`, `cwd`, etc.)
- Detect terminal via env vars (same logic as Swift version)
- Get git branch via `git rev-parse`
- Encode SessionEvent as JSON, send to Unix socket with send loop (handle short writes)
- Subcommands: `install` (write to `~/.claude/settings.json`), `uninstall`
- Always exit 0 to never block Claude Code

### 2. Socket Server (`internal/server`)

Listens on Unix domain socket, parses incoming JSON events.

Hardening (ported from Swift fixes):
- `NSLock` equivalent: Go's `sync.Mutex` for shutdown coordination
- 64KB max message size
- 5s read timeout per client (`SetReadDeadline`)
- EINTR handling (Go's net package handles this internally)
- Stale socket cleanup: test-connect before removing; don't delete live socket
- `umask(0o077)` before bind, restore after

### 3. Session Store (`internal/session`)

In-memory session map with cleanup timer.

- `map[string]SessionInfo` protected by `sync.RWMutex`
- Cleanup: waiting/done stale after 30min, active after 10min
- Persistence: JSON to `~/.cc-status/sessions.json` with 1s debounce
- Sorted output: waiting > done > active, then by last-updated desc

### 4. Tray UI (`internal/tray`)

Linux system tray using fyne-io/systray.

Tray icon states (priority order):
1. Any session waiting → 🟠 orange icon
2. Any session done → 🟢 green icon
3. Any session active → 🔵 blue icon
4. No sessions → ⚪ gray icon

Menu format:
```
🟠 cc-status · main — Waiting for input
🟢 rubato · feat/x — Done
🔵 myapp · develop — Working...
─────────────
Dismiss All
Quit
```

Click handler: detect terminal app from `terminal_id` prefix, run `xdg-open` or direct app launch.

### 5. Terminal Focus (`internal/tray`)

Simple app-level focus only:
- `ghostty:*` → launch/focus Ghostty
- `app:Cursor` → launch/focus Cursor
- Unknown → try common terminals in order (ghostty, kitty, alacritty, gnome-terminal, konsole)

Implementation: `exec.Command("xdg-open", ...)` or direct binary launch.

## Build & Install

```bash
# Build both binaries
cd cc-status-go
go build -o bin/cc-status-tray ./cmd/cc-status-tray
go build -o bin/cc-status-hook ./cmd/cc-status-hook

# Install hook into Claude Code settings
bin/cc-status-hook install

# Run tray app
bin/cc-status-tray

# Optional: autostart on login
cp cc-status-tray.desktop ~/.config/autostart/
```

## What Stays in Swift (macOS)

Everything under `Sources/` is unchanged:
- `CCStatus` — macOS menu bar app (AppKit, NSStatusBar, AppleScript focus)
- `CCStatusHook` — macOS hook binary (still works, users can choose Swift or Go hook)
- `CCStatusShared` — shared models and socket helpers

The Go hook is a drop-in replacement for the Swift hook — same socket protocol, same `settings.json` format. macOS users can use either.

## Out of Scope

- Windows support (no demand yet)
- Custom themes or color configuration
- Web-based UI alternative
- Multiple socket listeners (one tray app per user)
