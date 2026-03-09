import Foundation
import CCStatusShared

final class SocketServer: @unchecked Sendable {
    private let socketPath: String
    private let onEvent: @Sendable (SessionEvent) -> Void
    private let lock = NSLock()
    private var _socketFD: Int32 = -1
    private var _isRunning = false
    private let acceptQueue = DispatchQueue(label: "cc-status.socket-accept")
    private let clientQueue = DispatchQueue(label: "cc-status.socket-clients", attributes: .concurrent)

    private static let maxMessageSize = 65_536 // 64 KB
    private static let clientTimeout = timeval(tv_sec: 5, tv_usec: 0)

    private var socketFD: Int32 {
        get { lock.withLock { _socketFD } }
        set { lock.withLock { _socketFD = newValue } }
    }

    private var isRunning: Bool {
        get { lock.withLock { _isRunning } }
        set { lock.withLock { _isRunning = newValue } }
    }

    init(socketPath: String, onEvent: @escaping @Sendable (SessionEvent) -> Void) {
        self.socketPath = socketPath
        self.onEvent = onEvent
    }

    func start() {
        isRunning = true
        acceptQueue.async { [weak self] in
            self?.listen()
        }
    }

    func stop() {
        lock.withLock {
            _isRunning = false
            if _socketFD >= 0 {
                close(_socketFD)
                _socketFD = -1
            }
        }
        try? FileManager.default.removeItem(atPath: socketPath)
    }

    private func listen() {
        let dir = (socketPath as NSString).deletingLastPathComponent
        try? FileManager.default.createDirectory(atPath: dir, withIntermediateDirectories: true)

        if !cleanupStaleSocket() {
            // Another instance is actively listening — abort
            print("[CCStatus] Another instance is running, not starting server")
            isRunning = false
            return
        }

        var fd = socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else {
            print("[CCStatus] Failed to create socket")
            isRunning = false
            return
        }

        // Set umask before bind to avoid TOCTOU permission window
        let oldUmask = umask(0o077)
        var addr = makeUnixSocketAddress(path: socketPath)
        var bindResult = bindUnixSocket(fd: fd, addr: &addr)

        if bindResult != 0 {
            print("[CCStatus] Failed to bind socket: \(String(cString: strerror(errno)))")
            // Close and recreate fd before retry (POSIX: fd state undefined after failed bind)
            close(fd)
            if !cleanupStaleSocket() {
                umask(oldUmask)
                isRunning = false
                return
            }
            fd = socket(AF_UNIX, SOCK_STREAM, 0)
            guard fd >= 0 else {
                umask(oldUmask)
                isRunning = false
                return
            }
            addr = makeUnixSocketAddress(path: socketPath)
            bindResult = bindUnixSocket(fd: fd, addr: &addr)
            guard bindResult == 0 else {
                print("[CCStatus] Retry bind failed: \(String(cString: strerror(errno)))")
                close(fd)
                umask(oldUmask)
                isRunning = false
                return
            }
        }
        umask(oldUmask)

        // Belt-and-suspenders permission enforcement
        chmod(socketPath, 0o600)

        Darwin.listen(fd, 10)
        socketFD = fd
        print("[CCStatus] Listening on \(socketPath)")

        while isRunning {
            let clientFD = accept(fd, nil, nil)
            if clientFD < 0 {
                if !isRunning { break }
                continue
            }

            clientQueue.async { [weak self] in
                self?.handleClient(clientFD)
            }
        }
    }

    /// Remove stale socket file if no server is listening on it.
    /// Returns true if safe to proceed (socket removed or didn't exist).
    /// Returns false if another instance is actively listening.
    private func cleanupStaleSocket() -> Bool {
        guard FileManager.default.fileExists(atPath: socketPath) else { return true }

        let testFD = socket(AF_UNIX, SOCK_STREAM, 0)
        guard testFD >= 0 else {
            try? FileManager.default.removeItem(atPath: socketPath)
            return true
        }
        defer { close(testFD) }

        var addr = makeUnixSocketAddress(path: socketPath)
        let result = connectUnixSocket(fd: testFD, addr: &addr)
        if result != 0 {
            try? FileManager.default.removeItem(atPath: socketPath)
            print("[CCStatus] Removed stale socket file")
            return true
        } else {
            print("[CCStatus] Another instance is listening on \(socketPath)")
            return false
        }
    }

    private func handleClient(_ clientFD: Int32) {
        defer { close(clientFD) }

        var timeout = Self.clientTimeout
        setsockopt(clientFD, SOL_SOCKET, SO_RCVTIMEO, &timeout, socklen_t(MemoryLayout<timeval>.size))

        var data = Data()
        let bufferSize = 4096
        let buffer = UnsafeMutablePointer<UInt8>.allocate(capacity: bufferSize)
        defer { buffer.deallocate() }

        while true {
            let bytesRead = recv(clientFD, buffer, bufferSize, 0)
            if bytesRead == 0 { break }
            if bytesRead < 0 {
                if errno == EINTR { continue }
                break
            }
            data.append(buffer, count: bytesRead)
            if data.count > Self.maxMessageSize {
                print("[CCStatus] Client exceeded max message size, disconnecting")
                return
            }
        }

        guard !data.isEmpty else { return }

        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        decoder.dateDecodingStrategy = .secondsSince1970

        do {
            let event = try decoder.decode(SessionEvent.self, from: data)
            DispatchQueue.main.async { [weak self] in
                self?.onEvent(event)
            }
        } catch {
            print("[CCStatus] Failed to decode event: \(error)")
        }
    }
}
