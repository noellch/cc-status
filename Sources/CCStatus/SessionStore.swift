import Foundation
import CCStatusShared

struct SessionInfo: Sendable {
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
    @Published private(set) var sessions: [String: SessionInfo] = [:]

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
        Task { @MainActor in
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
}
