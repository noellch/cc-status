# Research: CC Status — Remaining Implementation

## Architecture Revision Based on Hooks API

### Key Finding: Hook Stdin JSON

Hooks receive **all context via stdin JSON**, not environment variables. Every event includes:
```json
{
  "session_id": "abc123",
  "cwd": "/path/to/project",
  "hook_event_name": "Stop",
  "permission_mode": "default"
}
```

This means our hook script needs to **read stdin and parse JSON**, not rely on env vars for session info.

### Revised Hook Event Mapping

| CC Hook Event | Our Status | Why |
|---|---|---|
| `SessionStart` | ACTIVE | New session started |
| `UserPromptSubmit` | ACTIVE | User sent prompt → CC is working |
| `Stop` | WAITING | CC finished responding, waiting for next user input |
| `Notification` (permission_prompt) | WAITING | CC needs permission approval |
| `Notification` (idle_prompt) | WAITING | CC idle, needs attention |
| `SessionEnd` | REMOVE | Session terminated, remove from list |

**Important change from original design:** We do NOT need `PreToolUse` for waiting state. The `Notification` event with `notification_type: "permission_prompt"` is what fires when CC pauses for tool approval. `Stop` fires when CC finishes its turn.

### Summary Extraction

| Event | Summary Source |
|---|---|
| `Stop` | `last_assistant_message` (truncated) |
| `Notification` | `message` field (e.g., "Claude needs permission to use Bash") |
| `UserPromptSubmit` | Static: "Working..." |
| `SessionStart` | Static: "Session started" |

### Hook Configuration

Hooks support `async: true` for command hooks — runs in background without blocking CC. This is critical for our use case since we don't want status reporting to slow down CC.

```json
{
  "hooks": {
    "Stop": [{
      "hooks": [{
        "type": "command",
        "command": "cc-status-hook",
        "async": true,
        "timeout": 5
      }]
    }]
  }
}
```

The hook script reads the full stdin JSON, which already contains `hook_event_name`, so we **don't need separate commands per event** — one script handles all events.

### Branch Detection

`cwd` is provided in stdin JSON. We still need to shell out to `git rev-parse` for branch name, but we have the correct directory.

### Terminal ID Detection

Still relies on environment variables (`$ITERM_SESSION_ID`, `$TERM_SESSION_ID`). These are inherited by the hook process from CC's shell environment.

## Current Codebase State

### What Works
- Menu bar app displays icon with badge count
- Socket server receives and decodes JSON events
- Dropdown panel lists sessions with status + summary
- State machine (ACTIVE/WAITING/DONE) works correctly

### What's Missing

1. **Hook script reads wrong format** — Currently expects CLI args (`cc-status-hook pre-tool-use`), needs to read stdin JSON instead
2. **No `install-hooks` command** — User must manually configure hooks
3. **No app bundle build script** — Must manually create .app bundle
4. **No session cleanup** — Sessions never get removed (no SessionEnd handling, no timeout cleanup)
5. **No `UserPromptSubmit` handling** — Can't detect when session goes from WAITING → ACTIVE
6. **State machine is incomplete** — Missing DONE→removed transition, missing SessionEnd
7. **No launch-at-login** — User must manually start the app

## Constraints

- **Swift 6 strict concurrency** — All shared mutable state must be properly isolated
- **App must run as .app bundle** — Bare binary can't access menu bar
- **Hook script must be fast** — async: true helps, but script should still be lightweight
- **Existing hooks** — `install-hooks` must merge with user's existing hook config, not overwrite

## Open Questions

1. Should we use HTTP hook type instead of command + socket? (Simpler but requires `allowedHttpHookUrls` config)
   → **Decision: Stay with command + socket.** More portable, no HTTP config needed.

2. Should DONE be a separate state or just map to WAITING?
   → **Decision: Keep DONE.** `Stop` with `last_assistant_message` containing completion language → DONE; otherwise → WAITING. For MVP, all Stop = WAITING is fine.
