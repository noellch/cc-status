# cc-status-go

Cross-platform system tray app and hook binary for monitoring Claude Code sessions.

Written in Go. Primary target is Linux; also runs on macOS for testing.

## Features

- System tray with colored emoji status dots (🟠 waiting, 🟢 done, 🔵 active)
- Session list with repo name, branch, and summary
- Click-to-focus terminal/IDE (macOS: `open -a`, Linux: binary launch)
- Parent PID liveness checks for fast orphan detection (~60s vs 10min fallback)
- Terminal detection: iTerm, Ghostty, Warp, VS Code, Cursor, Zed, WezTerm, kitty, and more

## Build

Requires Go 1.22+ and GTK3 dev libraries (Linux) or Cocoa (macOS).

```bash
# Linux: install GTK3 dev dependencies first
# Ubuntu/Debian: sudo apt install libgtk-3-dev libappindicator3-dev
# Fedora: sudo dnf install gtk3-devel libappindicator-gtk3-devel

make build
```

## Install

```bash
make install
```

This will:
1. Build both binaries
2. Copy them to `~/.local/bin/`
3. Register hooks in `~/.claude/settings.json`

## Run

```bash
cc-status-tray
```

## Autostart (Linux)

```bash
mkdir -p ~/.config/autostart
cp cc-status-tray.desktop ~/.config/autostart/
```

## Uninstall

```bash
make uninstall
```

## Test

```bash
go test ./...
```
