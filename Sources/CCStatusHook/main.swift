import Foundation
import CCStatusShared

// cc-status-hook <event-type>
// Reads CC hook context from stdin (JSON) and environment variables,
// then sends a SessionEvent to the menu bar app via Unix domain socket.

let args = CommandLine.arguments
guard args.count >= 2 else {
    print("Usage: cc-status-hook <pre-tool-use|stop|notification>")
    exit(1)
}

let eventType = args[1]

// Read hook input from stdin
let stdinData = FileHandle.standardInput.readDataToEndOfFile()
let hookInput = try? JSONSerialization.jsonObject(with: stdinData) as? [String: Any]

// Determine session ID — use CC's session ID from env or generate one
let sessionId = ProcessInfo.processInfo.environment["CLAUDE_SESSION_ID"]
    ?? ProcessInfo.processInfo.environment["CC_SESSION_ID"]
    ?? "unknown-\(ProcessInfo.processInfo.processIdentifier)"

// Detect terminal
let terminalId = detectTerminalId()

// Get working directory
let cwd = ProcessInfo.processInfo.environment["PWD"]
    ?? FileManager.default.currentDirectoryPath

// Get branch name
let branch = getCurrentBranch(cwd: cwd)

// Map event type to status
let status: SessionStatus
let summary: String

switch eventType {
case "pre-tool-use":
    let toolName = hookInput?["tool_name"] as? String ?? "unknown tool"
    let toolInput = hookInput?["tool_input"] as? [String: Any]

    // Check if this tool requires user confirmation
    // The hook fires before the tool runs — if it's showing to the user, it's waiting
    status = .waiting
    summary = formatToolSummary(toolName: toolName, toolInput: toolInput)

case "stop":
    status = .waiting
    let stopReason = hookInput?["stop_reason"] as? String ?? ""
    if stopReason == "end_turn" {
        summary = "Waiting for input"
    } else {
        summary = "Stopped: \(stopReason)"
    }

case "notification":
    let message = hookInput?["message"] as? String ?? "Task complete"
    status = .done
    summary = message

case "resume":
    status = .active
    summary = "Working..."

default:
    print("Unknown event type: \(eventType)")
    exit(1)
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
    if let iterm = env["ITERM_SESSION_ID"] {
        return "iterm:\(iterm)"
    }
    if let term = env["TERM_SESSION_ID"] {
        return "terminal:\(term)"
    }
    if let warp = env["WARP_SESSION_ID"] {
        return "warp:\(warp)"
    }
    // Fallback: use TTY
    if let tty = env["TTY"] {
        return "terminal:\(tty)"
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
        print("[cc-status-hook] Failed to encode event")
        exit(1)
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
