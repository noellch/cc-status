import Foundation
import CCStatusShared

// cc-status-hook
// Claude Code hooks pass context via stdin JSON.
// This script reads that JSON, maps the hook event to a SessionEvent,
// and sends it to the menu bar app via Unix domain socket.
// Always exits 0 to never block Claude Code.

let args = CommandLine.arguments

// Handle install/uninstall subcommands (placeholder for Task 4)
if args.count >= 2 {
    switch args[1] {
    case "install":
        do {
            try HookInstaller.install()
        } catch {
            fputs("Error installing hooks: \(error.localizedDescription)\n", stderr)
            exit(1)
        }
        exit(0)
    case "uninstall":
        do {
            try HookInstaller.uninstall()
        } catch {
            fputs("Error uninstalling hooks: \(error.localizedDescription)\n", stderr)
            exit(1)
        }
        exit(0)
    default:
        break
    }
}

// --- Stdin JSON hook flow ---

// Read hook input from stdin
let stdinData = FileHandle.standardInput.readDataToEndOfFile()
guard !stdinData.isEmpty,
      let hookInput = try? JSONSerialization.jsonObject(with: stdinData) as? [String: Any],
      let hookEventName = hookInput["hook_event_name"] as? String else {
    // Empty stdin or invalid JSON — silently exit
    exit(0)
}

// Get session_id from stdin JSON
let sessionId = hookInput["session_id"] as? String
    ?? "unknown-\(ProcessInfo.processInfo.processIdentifier)"

// Get cwd from stdin JSON
let cwd = hookInput["cwd"] as? String
    ?? FileManager.default.currentDirectoryPath

// Detect terminal
let terminalId = detectTerminalId()

// Get branch name from cwd
let branch = getCurrentBranch(cwd: cwd)

// Route based on hook_event_name
let status: SessionStatus
let summary: String

switch hookEventName {
case "SessionStart":
    status = .active
    summary = "Session started"

case "UserPromptSubmit":
    status = .active
    summary = "Working..."

case "Stop":
    status = .waiting
    if let lastMessage = hookInput["last_assistant_message"] as? String, !lastMessage.isEmpty {
        summary = lastMessage.count > 80 ? String(lastMessage.prefix(80)) + "..." : lastMessage
    } else {
        summary = "Waiting for input"
    }

case "Notification":
    let notificationType = hookInput["notification_type"] as? String ?? ""
    if notificationType == "permission_prompt" || notificationType == "idle_prompt" {
        status = .waiting
        summary = hookInput["message"] as? String ?? "Needs attention"
    } else {
        // Other notification types — silently exit
        exit(0)
    }

case "SessionEnd":
    status = .remove
    summary = ""

default:
    // Unknown event — silently exit
    exit(0)
}

// Build event
let event = SessionEvent(
    sessionId: sessionId,
    event: status,
    cwd: cwd,
    branch: branch,
    summary: summary,
    terminalId: terminalId
)

// Send to socket
sendToSocket(event: event)

// MARK: - Helpers

func detectTerminalId() -> String? {
    let env = ProcessInfo.processInfo.environment

    // Specific terminal apps with dedicated session IDs
    if let iterm = env["ITERM_SESSION_ID"] {
        return "iterm:\(iterm)"
    }
    if let term = env["TERM_SESSION_ID"] {
        return "terminal:\(term)"
    }
    if let warp = env["WARP_SESSION_ID"] {
        return "warp:\(warp)"
    }
    if env["GHOSTTY_BIN_DIR"] != nil || env["TERM_PROGRAM"] == "ghostty" {
        let windowId = env["GHOSTTY_WINDOW_ID"] ?? ""
        return "ghostty:\(windowId)"
    }

    // macOS injects __CFBundleIdentifier into child processes — most reliable detection
    let bundleId = env["__CFBundleIdentifier"] ?? ""
    let bundleToApp: [String: String] = [
        "com.todesktop.230313mzl4w4u92": "Cursor",
        "com.microsoft.VSCode":           "Visual Studio Code",
        "com.microsoft.VSCodeInsiders":   "Visual Studio Code - Insiders",
        "dev.zed.Zed":                    "Zed",
        "com.github.wez.wezterm":         "WezTerm",
        "net.kovidgoyal.kitty":           "kitty",
        "io.alacritty":                   "Alacritty",
        "co.zeit.hyper":                  "Hyper",
    ]
    if let appName = bundleToApp[bundleId] {
        return "app:\(appName)"
    }

    // Fallback: IDE-specific env vars
    // When VSCODE_PID is set, use it to find the actual app (Cursor vs VS Code)
    if env["CURSOR_TRACE_ID"] != nil || env["TERM_PROGRAM"] == "cursor" {
        return "app:Cursor"
    }
    if let vscodePid = env["VSCODE_PID"], let pid = Int32(vscodePid) {
        // Resolve actual app name from the VSCODE_PID process
        if let appName = appNameFromPid(pid) {
            return "app:\(appName)"
        }
        return "app:Visual Studio Code"
    }
    if env["TERM_PROGRAM"] == "vscode" {
        // No VSCODE_PID — check running apps to guess
        if isAppRunning(bundleId: "com.todesktop.230313mzl4w4u92") {
            return "app:Cursor"
        }
        return "app:Visual Studio Code"
    }

    // Other terminals by TERM_PROGRAM
    let termProgramMap: [String: String] = [
        "WezTerm":   "WezTerm",
        "zed":       "Zed",
        "Hyper":     "Hyper",
        "kitty":     "kitty",
        "Alacritty": "Alacritty",
    ]
    if let termProgram = env["TERM_PROGRAM"], let appName = termProgramMap[termProgram] {
        return "app:\(appName)"
    }

    if env["TERM_PROGRAM"] == "tmux" {
        if let tty = env["TTY"] { return "terminal:\(tty)" }
        return nil
    }

    if let tty = env["TTY"] {
        return "terminal:\(tty)"
    }
    if let termProgram = env["TERM_PROGRAM"] {
        return "app:\(termProgram)"
    }
    return nil
}

/// Walk up the process tree from a PID to find the owning .app bundle name.
func appNameFromPid(_ pid: Int32) -> String? {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/bin/ps")
    process.arguments = ["-p", "\(pid)", "-o", "comm="]
    let pipe = Pipe()
    process.standardOutput = pipe
    process.standardError = FileHandle.nullDevice
    do {
        try process.run()
        process.waitUntilExit()
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        let comm = String(data: data, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        // comm is the executable path, e.g. /Applications/Cursor.app/Contents/MacOS/Cursor
        if comm.contains("Cursor.app") { return "Cursor" }
        if comm.contains("Visual Studio Code.app") { return "Visual Studio Code" }
        if comm.contains("Code.app") { return "Visual Studio Code" }
        if comm.contains("Windsurf.app") { return "Windsurf" }
        // Fallback: extract app name from .app path
        if let range = comm.range(of: #"/([^/]+)\.app/"#, options: .regularExpression) {
            return String(comm[range]).replacingOccurrences(of: "/", with: "").replacingOccurrences(of: ".app", with: "")
        }
    } catch {}
    return nil
}

/// Check if an app with the given bundle ID is currently running.
func isAppRunning(bundleId: String) -> Bool {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/pgrep")
    process.arguments = ["-f", bundleId]
    process.standardOutput = FileHandle.nullDevice
    process.standardError = FileHandle.nullDevice
    do {
        try process.run()
        process.waitUntilExit()
        return process.terminationStatus == 0
    } catch {
        return false
    }
}

func getCurrentBranch(cwd: String) -> String {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/git")
    process.arguments = ["-C", cwd, "rev-parse", "--abbrev-ref", "HEAD"]
    let pipe = Pipe()
    process.standardOutput = pipe
    process.standardError = FileHandle.nullDevice
    do {
        try process.run()
        process.waitUntilExit()
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        return String(data: data, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    } catch {
        return ""
    }
}

func sendToSocket(event: SessionEvent) {
    let encoder = JSONEncoder()
    encoder.keyEncodingStrategy = .convertToSnakeCase
    encoder.dateEncodingStrategy = .secondsSince1970

    guard let data = try? encoder.encode(event) else {
        // Failed to encode — silently exit
        exit(0)
    }

    let socketFD = socket(AF_UNIX, SOCK_STREAM, 0)
    guard socketFD >= 0 else {
        // Menu bar app might not be running — silently exit
        exit(0)
    }
    defer { close(socketFD) }

    var addr = makeUnixSocketAddress(path: CCStatusConfig.socketPath)
    let connectResult = connectUnixSocket(fd: socketFD, addr: &addr)

    guard connectResult == 0 else {
        // Menu bar app not running — silently exit
        exit(0)
    }

    data.withUnsafeBytes { bufferPtr in
        var remaining = data.count
        var offset = 0
        while remaining > 0 {
            let sent = send(socketFD, bufferPtr.baseAddress! + offset, remaining, 0)
            if sent <= 0 { break }
            offset += sent
            remaining -= sent
        }
    }
}
