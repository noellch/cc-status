import AppKit

enum TerminalJumper {
    static func focusTerminal(terminalId: String) {
        if terminalId.hasPrefix("iterm:") {
            focusITerm(sessionId: String(terminalId.dropFirst("iterm:".count)))
        } else if terminalId.hasPrefix("terminal:") {
            focusTerminalApp(sessionId: String(terminalId.dropFirst("terminal:".count)))
        } else if terminalId.hasPrefix("ghostty:") {
            openApp("Ghostty")
        } else if terminalId.hasPrefix("warp:") {
            openApp("Warp")
        } else if terminalId.hasPrefix("app:") {
            let appName = String(terminalId.dropFirst("app:".count))
            openApp(appName)
        } else {
            openApp("Ghostty")
        }
    }

    static func focusAnyTerminal() {
        let knownApps = ["Ghostty", "iTerm", "Terminal", "Warp"]
        for name in knownApps {
            let bundleId: String
            switch name {
            case "Ghostty": bundleId = "com.mitchellh.ghostty"
            case "iTerm": bundleId = "com.googlecode.iterm2"
            case "Terminal": bundleId = "com.apple.Terminal"
            case "Warp": bundleId = "dev.warp.Warp-Stable"
            default: continue
            }
            if NSRunningApplication.runningApplications(withBundleIdentifier: bundleId).first != nil {
                openApp(name)
                return
            }
        }
    }

    /// Use `open -a` which reliably activates apps even from accessory (LSUIElement) apps
    private static func openApp(_ name: String) {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/open")
        process.arguments = ["-a", name]
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        try? process.run()
    }

    private static func focusITerm(sessionId: String) {
        let script = """
        tell application "iTerm2"
            activate
            repeat with aWindow in windows
                repeat with aTab in tabs of aWindow
                    repeat with aSession in sessions of aTab
                        if unique ID of aSession contains "\(sessionId)" then
                            select aTab
                            select aWindow
                            return
                        end if
                    end repeat
                end repeat
            end repeat
        end tell
        """
        runAppleScript(script)
    }

    private static func focusTerminalApp(sessionId: String) {
        let script = """
        tell application "Terminal"
            activate
            repeat with aWindow in windows
                repeat with aTab in tabs of aWindow
                    if tty of aTab contains "\(sessionId)" then
                        set selected tab of aWindow to aTab
                        set index of aWindow to 1
                        return
                    end if
                end repeat
            end repeat
        end tell
        """
        runAppleScript(script)
    }

    private static func runAppleScript(_ source: String) {
        guard let script = NSAppleScript(source: source) else { return }
        var error: NSDictionary?
        script.executeAndReturnError(&error)
        if let error {
            print("[CCStatus] AppleScript error: \(error)")
        }
    }
}
