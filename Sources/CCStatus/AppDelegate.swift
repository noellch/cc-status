import AppKit
import CCStatusShared

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusBarController: StatusBarController?
    private var socketServer: SocketServer?
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
    }

    func applicationWillTerminate(_ notification: Notification) {
        socketServer?.stop()
    }
}
