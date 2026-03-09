import Foundation

public enum SessionStatus: String, Codable, Sendable {
    case active
    case waiting
    case done
}

public struct SessionEvent: Codable, Sendable {
    public let sessionId: String
    public let event: SessionStatus
    public let cwd: String
    public let branch: String
    public let summary: String
    public let terminalId: String?
    public let timestamp: Date

    public init(
        sessionId: String,
        event: SessionStatus,
        cwd: String,
        branch: String,
        summary: String,
        terminalId: String?,
        timestamp: Date = Date()
    ) {
        self.sessionId = sessionId
        self.event = event
        self.cwd = cwd
        self.branch = branch
        self.summary = summary
        self.terminalId = terminalId
        self.timestamp = timestamp
    }
}

public struct CCStatusConfig: Sendable {
    public static let socketDir = FileManager.default.homeDirectoryForCurrentUser
        .appendingPathComponent(".cc-status")
    public static let socketPath = socketDir.appendingPathComponent("cc-status.sock").path
}
