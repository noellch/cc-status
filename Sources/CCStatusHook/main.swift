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
        return "ghostty:"
    }

    // IDE integrated terminals — detect via TERM_PROGRAM or known env vars
    if env["VSCODE_PID"] != nil || env["TERM_PROGRAM"] == "vscode" {
        return "app:Visual Studio Code"
    }
    if env["CURSOR_TRACE_ID"] != nil || env["TERM_PROGRAM"] == "cursor" {
        return "app:Cursor"
    }
    if env["TERM_PROGRAM"] == "WezTerm" {
        return "app:WezTerm"
    }
    if env["TERM_PROGRAM"] == "zed" {
        return "app:Zed"
    }
    if env["TERM_PROGRAM"] == "Hyper" {
        return "app:Hyper"
    }
    if env["TERM_PROGRAM"] == "kitty" {
        return "app:kitty"
    }
    if env["TERM_PROGRAM"] == "Alacritty" {
        return "app:Alacritty"
    }
    if env["TERM_PROGRAM"] == "tmux" {
        // tmux inside another terminal — try to detect the outer terminal
        if let tty = env["TTY"] {
            return "terminal:\(tty)"
        }
        return nil
    }

    if let tty = env["TTY"] {
        return "terminal:\(tty)"
    }
    // Final fallback
    if let termProgram = env["TERM_PROGRAM"] {
        return "app:\(termProgram)"
    }
    return nil
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

func formatToolSummary(toolName: String, toolInput: [String: Any]?) -> String {
    switch toolName {
    case "Bash":
        if let cmd = toolInput?["command"] as? String {
            let truncated = cmd.count > 80 ? String(cmd.prefix(80)) + "..." : cmd
            return "Confirm: \(truncated)"
        }
        return "Confirm: run command"
    case "Write":
        if let path = toolInput?["file_path"] as? String {
            return "Confirm: write \((path as NSString).lastPathComponent)"
        }
        return "Confirm: write file"
    case "Edit":
        if let path = toolInput?["file_path"] as? String {
            return "Confirm: edit \((path as NSString).lastPathComponent)"
        }
        return "Confirm: edit file"
    default:
        return "Confirm: \(toolName)"
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
        _ = send(socketFD, bufferPtr.baseAddress!, data.count, 0)
    }
}
