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
            openApp(sanitizeAppName(appName))
        } else {
            openApp("Ghostty")
        }
    }

    static func focusAnyTerminal() {
        for (name, bundleId) in knownTerminals {
            if NSRunningApplication.runningApplications(withBundleIdentifier: bundleId).first != nil {
                openApp(name)
                return
            }
        }
    }

    // MARK: - Known Terminals

    private static let knownTerminals: [(String, String)] = [
        ("Ghostty", "com.mitchellh.ghostty"),
        ("iTerm",   "com.googlecode.iterm2"),
        ("Terminal", "com.apple.Terminal"),
        ("Warp",    "dev.warp.Warp-Stable"),
    ]

    // MARK: - Sanitization

    /// Strip characters that could break shell or AppleScript injection.
    private static func sanitizeAppName(_ name: String) -> String {
        // Only allow alphanumeric, spaces, hyphens, dots
        String(name.filter { $0.isLetter || $0.isNumber || $0 == " " || $0 == "-" || $0 == "." })
    }

    /// Escape a string for safe interpolation inside AppleScript double-quoted strings.
    private static func escapeForAppleScript(_ input: String) -> String {
        input
            .replacingOccurrences(of: "\\", with: "\\\\")
            .replacingOccurrences(of: "\"", with: "\\\"")
    }

    // MARK: - App Activation

    /// Use `open -a` which reliably activates apps even from accessory (LSUIElement) apps
    private static func openApp(_ name: String) {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/open")
        process.arguments = ["-a", name]
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        try? process.run()
    }

    // MARK: - AppleScript Focus

    private static func focusITerm(sessionId: String) {
        let safe = escapeForAppleScript(sessionId)
        let script = """
        tell application "iTerm2"
            activate
            repeat with aWindow in windows
                repeat with aTab in tabs of aWindow
                    repeat with aSession in sessions of aTab
                        if unique ID of aSession contains "\(safe)" then
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
        let safe = escapeForAppleScript(sessionId)
        let script = """
        tell application "Terminal"
            activate
            repeat with aWindow in windows
                repeat with aTab in tabs of aWindow
                    if tty of aTab contains "\(safe)" then
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
