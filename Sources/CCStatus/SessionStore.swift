import Foundation
import CCStatusShared

struct SessionInfo: Sendable, Codable {
    let sessionId: String
    var status: SessionStatus
    var cwd: String
    var branch: String
    var summary: String
    var terminalId: String?
    var lastUpdated: Date
}

@MainActor
final class SessionStore: ObservableObject {
    @Published private(set) var sessions: [String: SessionInfo] = [:] {
        didSet { scheduleSave() }
    }

    private static let persistenceURL: URL = {
        CCStatusConfig.socketDir.appendingPathComponent("sessions.json")
    }()
    private var saveTask: Task<Void, Never>?

    var waitingCount: Int {
        sessions.values.filter { $0.status == .waiting }.count
    }

    var doneCount: Int {
        sessions.values.filter { $0.status == .done }.count
    }

    var needsAttentionCount: Int {
        waitingCount + doneCount
    }

    var sortedSessions: [SessionInfo] {
        sessions.values.sorted { a, b in
            let order: (SessionStatus) -> Int = { status in
                switch status {
                case .waiting: return 0
                case .done: return 1
                case .active: return 2
                case .remove: return 3
                }
            }
            if order(a.status) != order(b.status) {
                return order(a.status) < order(b.status)
            }
            return a.lastUpdated > b.lastUpdated
        }
    }

    func handleEvent(_ event: SessionEvent) {
        if event.event == .remove {
            sessions.removeValue(forKey: event.sessionId)
        } else {
            sessions[event.sessionId] = SessionInfo(
                sessionId: event.sessionId,
                status: event.event,
                cwd: event.cwd,
                branch: event.branch,
                summary: event.summary,
                terminalId: event.terminalId,
                lastUpdated: event.timestamp
            )
        }
    }

    func dismissDone() {
        sessions = sessions.filter { $0.value.status != .done }
    }

    func dismissAll() {
        sessions.removeAll()
    }

    func removeSession(_ id: String) {
        sessions.removeValue(forKey: id)
    }

    /// Remove stale sessions:
    /// - waiting/done: no update for 30+ minutes
    /// - active: no update for 10+ minutes (likely orphaned by killed terminal)
    func cleanupStaleSessions() {
        let now = Date()
        let idleThreshold = now.addingTimeInterval(-30 * 60)
        let activeThreshold = now.addingTimeInterval(-10 * 60)
        sessions = sessions.filter { _, session in
            switch session.status {
            case .waiting, .done:
                return session.lastUpdated > idleThreshold
            case .active:
                return session.lastUpdated > activeThreshold
            case .remove:
                return false
            }
        }
    }

    /// Extract display name from cwd (last path component)
    func displayName(for session: SessionInfo) -> String {
        let repo = (session.cwd as NSString).lastPathComponent
        if session.branch.isEmpty {
            return repo
        }
        return "\(repo) (\(session.branch))"
    }

    // MARK: - Persistence

    func loadFromDisk() {
        let url = Self.persistenceURL
        guard FileManager.default.fileExists(atPath: url.path),
              let data = try? Data(contentsOf: url) else { return }

        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .secondsSince1970
        guard let saved = try? decoder.decode([String: SessionInfo].self, from: data) else { return }

        // Only restore non-stale sessions
        sessions = saved
        cleanupStaleSessions()
    }

    private func scheduleSave() {
        saveTask?.cancel()
        saveTask = Task {
            try? await Task.sleep(for: .seconds(1))
            guard !Task.isCancelled else { return }
            saveToDisk()
        }
    }

    private func saveToDisk() {
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .secondsSince1970
        guard let data = try? encoder.encode(sessions) else { return }

        let url = Self.persistenceURL
        let dir = url.deletingLastPathComponent()
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try? data.write(to: url, options: .atomic)
    }
}
