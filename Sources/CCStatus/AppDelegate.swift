import AppKit
import CCStatusShared

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusBarController: StatusBarController?
    private var socketServer: SocketServer?
    private var cleanupTimer: Timer?
    private let sessionStore = SessionStore()

    func applicationDidFinishLaunching(_ notification: Notification) {
        // Hide dock icon — menu bar only app
        NSApp.setActivationPolicy(.accessory)

        statusBarController = StatusBarController(sessionStore: sessionStore)

        socketServer = SocketServer(socketPath: CCStatusConfig.socketPath) { [weak self] event in
            Task { @MainActor in
                self?.sessionStore.handleEvent(event)
            }
        }
        socketServer?.start()

        cleanupTimer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in
            Task { @MainActor in
                self?.sessionStore.cleanupStaleSessions()
            }
        }
    }

    func applicationWillTerminate(_ notification: Notification) {
        cleanupTimer?.invalidate()
        socketServer?.stop()
    }
}
