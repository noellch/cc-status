import Foundation
import CCStatusShared

final class SocketServer: @unchecked Sendable {
    private let socketPath: String
    private let onEvent: @Sendable (SessionEvent) -> Void
    private var fileHandle: FileHandle?
    private var socketFD: Int32 = -1
    private var isRunning = false
    private let acceptQueue = DispatchQueue(label: "cc-status.socket-accept")
    private let clientQueue = DispatchQueue(label: "cc-status.socket-clients", attributes: .concurrent)

    init(socketPath: String, onEvent: @escaping @Sendable (SessionEvent) -> Void) {
        self.socketPath = socketPath
        self.onEvent = onEvent
    }

    func start() {
        acceptQueue.async { [weak self] in
            self?.listen()
        }
    }

    func stop() {
        isRunning = false
        if socketFD >= 0 {
            close(socketFD)
            socketFD = -1
        }
        try? FileManager.default.removeItem(atPath: socketPath)
    }

    private func listen() {
        let dir = (socketPath as NSString).deletingLastPathComponent
        try? FileManager.default.createDirectory(atPath: dir, withIntermediateDirectories: true)

        // Remove stale socket file
        try? FileManager.default.removeItem(atPath: socketPath)

        socketFD = socket(AF_UNIX, SOCK_STREAM, 0)
        guard socketFD >= 0 else {
            print("[CCStatus] Failed to create socket")
            return
        }

        var addr = makeUnixSocketAddress(path: socketPath)
        let bindResult = bindUnixSocket(fd: socketFD, addr: &addr)
        guard bindResult == 0 else {
            print("[CCStatus] Failed to bind socket: \(String(cString: strerror(errno)))")
            return
        }

        Darwin.listen(socketFD, 10)
        isRunning = true
        print("[CCStatus] Listening on \(socketPath)")

        while isRunning {
            let clientFD = accept(socketFD, nil, nil)
            guard clientFD >= 0 else { continue }

            clientQueue.async { [weak self] in
                self?.handleClient(clientFD)
            }
        }
    }

    private func handleClient(_ clientFD: Int32) {
        defer { close(clientFD) }

        var data = Data()
        let bufferSize = 4096
        let buffer = UnsafeMutablePointer<UInt8>.allocate(capacity: bufferSize)
        defer { buffer.deallocate() }

        while true {
            let bytesRead = recv(clientFD, buffer, bufferSize, 0)
            if bytesRead <= 0 { break }
            data.append(buffer, count: bytesRead)
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
