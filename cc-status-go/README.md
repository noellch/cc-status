# cc-status-go

Linux system tray app and cross-platform hook binary for monitoring Claude Code sessions.

Companion to the macOS Swift version in the parent directory.

## Build

Requires Go 1.22+ and GTK3 dev libraries (Linux) or Cocoa (macOS).

```bash
# Linux: install GTK3 dev dependencies first
# Ubuntu/Debian: sudo apt install libgtk-3-dev libappindicator3-dev
# Fedora: sudo dnf install gtk3-devel libappindicator-gtk3-devel

go build -o bin/cc-status-tray ./cmd/cc-status-tray
go build -o bin/cc-status-hook ./cmd/cc-status-hook
```

## Install

```bash
# Install hook into Claude Code
./bin/cc-status-hook install

# Copy binaries
mkdir -p ~/.local/bin
cp bin/cc-status-tray bin/cc-status-hook ~/.local/bin/
```

## Run

```bash
cc-status-tray
```

## Autostart (Linux)

```bash
mkdir -p ~/.config/autostart
cat > ~/.config/autostart/cc-status-tray.desktop << 'EOF'
[Desktop Entry]
Type=Application
Name=CC Status
Exec=$HOME/.local/bin/cc-status-tray
Terminal=false
EOF
```

## Uninstall

```bash
cc-status-hook uninstall
rm ~/.local/bin/cc-status-tray ~/.local/bin/cc-status-hook
rm ~/.config/autostart/cc-status-tray.desktop
```

## Test

```bash
go test ./... -v
```
