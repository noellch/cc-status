import Foundation

/// Cross-platform process liveness checks to detect orphaned sessions.
enum ProcessChecker {
    /// Check if a process with the given PID is still running
    /// AND was started at the given startTime (to guard against PID reuse).
    /// If startTime is nil, only the PID existence check is performed.
    static func isAlive(pid: Int, startTime: String?) -> Bool {
        guard pid > 0 else { return false }

        // Signal 0 checks if the process exists without actually signaling it.
        let exists = kill(Int32(pid), 0) == 0
        if !exists { return false }

        // If no start time to verify, PID exists = alive.
        guard let expectedStart = startTime, !expectedStart.isEmpty else {
            return true
        }

        // Verify start time matches (guards against PID reuse).
        guard let currentStart = getStartTime(pid: pid), currentStart == expectedStart else {
            // Start time mismatch or unavailable — assume PID was reused.
            // If we can't get the start time, err on the side of keeping the session
            // (time-based cleanup will handle it eventually).
            return getStartTime(pid: pid) == nil
        }

        return true
    }

    /// Get the start time of a process using `ps -p <pid> -o lstart=`.
    /// Works on both macOS and Linux. Returns nil if process doesn't exist.
    static func getStartTime(pid: Int) -> String? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/ps")
        process.arguments = ["-p", "\(pid)", "-o", "lstart="]
        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = FileHandle.nullDevice
        do {
            try process.run()
            process.waitUntilExit()
            let data = pipe.fileHandleForReading.readDataToEndOfFile()
            let output = String(data: data, encoding: .utf8)?
                .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            return output.isEmpty ? nil : output
        } catch {
            return nil
        }
    }
}
