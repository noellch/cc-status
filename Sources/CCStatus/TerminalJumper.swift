import AppKit

enum TerminalJumper {
    static func focusTerminal(terminalId: String) {
        // Parse terminal type from ID prefix
        if terminalId.hasPrefix("iterm:") {
            focusITerm(sessionId: String(terminalId.dropFirst("iterm:".count)))
        } else if terminalId.hasPrefix("terminal:") {
            focusTerminalApp(sessionId: String(terminalId.dropFirst("terminal:".count)))
        } else if terminalId.hasPrefix("warp:") {
            // Warp — best effort, activate the app
            activateApp(bundleId: "dev.warp.Warp-Stable")
        } else {
            // Fallback: try to activate by raw bundle ID or just bring terminal forward
            activateApp(bundleId: "com.googlecode.iterm2")
        }
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

    private static func activateApp(bundleId: String) {
        if let app = NSRunningApplication.runningApplications(withBundleIdentifier: bundleId).first {
            app.activate()
        }
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
