# CC Status — macOS Menu Bar Tool for Claude Code

## Problem

When running 4+ concurrent Claude Code sessions, there is no way to know which sessions need human action (approve, confirm, review) or have completed tasks without manually checking each terminal.

## Solution

A native macOS menu bar app that receives real-time status updates from Claude Code via hooks, displaying which sessions need attention.

## Architecture

```
CC Session 1 ──┐
CC Session 2 ──┤── hook script ── JSON ──▶ Unix Domain Socket ──▶ Menu Bar App
CC Session N ──┘                          (~/.cc-status/sock)     (NSStatusItem)
```

Three components:

1. **Hook Script (`cc-status-hook`)** — Lightweight CLI installed alongside the app. Configured in CC hooks settings. On each CC event, reads hook env vars, detects terminal ID, sends JSON to Unix domain socket.
2. **Menu Bar App (`cc-status`)** — Swift macOS app using `NSStatusItem`. Listens on socket, maintains session state, renders icon + dropdown panel.
3. **State Store** — In-memory session table keyed by `session_id`.

Communication via Unix domain socket at `~/.cc-status/cc-status.sock` — local only, no open ports, high performance.

## Hook Events

| CC Hook Event | Mapped Status | Description |
|---|---|---|
| `PreToolUse` (user confirmation pending) | `WAITING` | CC paused, waiting for tool approval |
| `Stop` | `WAITING` | CC turn ended, waiting for user input |
| `Notification` (task complete) | `DONE` | Task finished |
| `PostToolUse` / resumed | `ACTIVE` | CC working |

## Session State Machine

```
ACTIVE ──▶ WAITING (Stop / PreToolUse pending confirmation)
WAITING ──▶ ACTIVE (user responded, CC resumed)
ACTIVE ──▶ DONE (Notification: task complete)
```

## Hook JSON Payload

```json
{
  "session_id": "abc123",
  "event": "waiting | active | done",
  "cwd": "/Users/noel/Crescendolab/rubato",
  "branch": "feat/add-identify",
  "summary": "Waiting for confirmation: git push origin main",
  "terminal_id": "com.apple.Terminal:window1:tab3",
  "timestamp": 1741520400
}
```

- `session_id`: Unique per CC session (generated at session start by hook)
- `terminal_id`: Detected from env vars (`$TERM_SESSION_ID`, `$ITERM_SESSION_ID`, etc.) for jump-to-terminal
- `summary`: Human-readable description of what CC is waiting for or completed

## Menu Bar UI

### Icon States

| State | Icon |
|---|---|
| All sessions ACTIVE | Grey dot |
| Any WAITING | Orange dot + badge count |
| Any DONE (no WAITING) | Green dot + badge count |
| WAITING + DONE mixed | Orange dot + total badge |
| No sessions | Grey hollow dot |

### Dropdown Panel

```
┌─────────────────────────────────────────┐
│  CC Status                    Settings  │
├─────────────────────────────────────────┤
│  [orange] rubato (feat/add-identify)    │
│     Waiting: git push origin main       │
│                                         │
│  [green] dolce (fix/timeout-bug)        │
│     Task complete                       │
│                                         │
│  [grey] grazioso (main)                 │
│     Working...                          │
├─────────────────────────────────────────┤
│  Dismiss All Done                       │
└─────────────────────────────────────────┘
```

### Interactions

- **Click session item** → Focus the corresponding terminal window (AppleScript / `open` command)
- **Dismiss All Done** → Clear all DONE sessions from list, update badge
- **Auto-cleanup** → Remove sessions 5 min after CC process exits
- **Settings** → Launch at login, socket path config

## Installation

### 1. Build & Install

```bash
git clone <repo>
cd cc-status
swift build -c release
cp .build/release/cc-status /usr/local/bin/
cp .build/release/cc-status-hook /usr/local/bin/
```

### 2. Configure CC Hooks (one command)

```bash
cc-status install-hooks
```

Adds to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "",
      "hooks": [{ "type": "command", "command": "cc-status-hook pre-tool-use" }]
    }],
    "Stop": [{
      "matcher": "",
      "hooks": [{ "type": "command", "command": "cc-status-hook stop" }]
    }],
    "Notification": [{
      "matcher": "",
      "hooks": [{ "type": "command", "command": "cc-status-hook notification" }]
    }]
  }
}
```

## Project Structure

```
cc-status/
├── Package.swift
├── Sources/
│   ├── CCStatus/            # Menu Bar App
│   │   ├── App.swift
│   │   ├── StatusBarController.swift
│   │   ├── SessionStore.swift
│   │   └── SocketServer.swift
│   └── CCStatusHook/        # Hook CLI Tool
│       └── main.swift
├── docs/
│   └── plans/
│       └── 2026-03-09-cc-status-design.md
└── README.md
```

Two targets in one Swift Package: the menu bar app and the hook CLI tool.

## Terminal Jump Support

| Terminal App | ID Source | Jump Method |
|---|---|---|
| Terminal.app | `$TERM_SESSION_ID` | AppleScript: `tell application "Terminal"` |
| iTerm2 | `$ITERM_SESSION_ID` | AppleScript: `tell application "iTerm2"` |
| Warp | `$WARP_SESSION_ID` | AppleScript (if supported) |
| Other | fallback: PID-based | Best effort |

## Future Considerations (Not in MVP)

- Token usage display per session
- Session duration tracking
- Sound alerts (configurable)
- Homebrew formula for easy distribution
- Keyboard shortcut to cycle through WAITING sessions
