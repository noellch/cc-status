import Foundation

public func makeUnixSocketAddress(path: String) -> sockaddr_un {
    var addr = sockaddr_un()
    addr.sun_family = sa_family_t(AF_UNIX)
    let pathBytes = path.utf8CString
    let maxLen = MemoryLayout.size(ofValue: addr.sun_path)
    withUnsafeMutableBytes(of: &addr.sun_path) { rawBuf in
        let count = min(pathBytes.count, maxLen)
        for i in 0..<count {
            rawBuf[i] = UInt8(bitPattern: pathBytes[i])
        }
    }
    return addr
}

public func connectUnixSocket(fd: Int32, addr: inout sockaddr_un) -> Int32 {
    withUnsafePointer(to: &addr) { ptr in
        ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockaddrPtr in
            connect(fd, sockaddrPtr, socklen_t(MemoryLayout<sockaddr_un>.size))
        }
    }
}

public func bindUnixSocket(fd: Int32, addr: inout sockaddr_un) -> Int32 {
    withUnsafePointer(to: &addr) { ptr in
        ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockaddrPtr in
            bind(fd, sockaddrPtr, socklen_t(MemoryLayout<sockaddr_un>.size))
        }
    }
}
