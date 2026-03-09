// swift-tools-version: 6.0

import PackageDescription

let package = Package(
    name: "CCStatus",
    platforms: [
        .macOS(.v13)
    ],
    targets: [
        .target(
            name: "CCStatusShared",
            path: "Sources/CCStatusShared"
        ),
        .executableTarget(
            name: "CCStatus",
            dependencies: ["CCStatusShared"],
            path: "Sources/CCStatus"
        ),
        .executableTarget(
            name: "CCStatusHook",
            dependencies: ["CCStatusShared"],
            path: "Sources/CCStatusHook"
        ),
    ]
)
