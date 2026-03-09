import AppKit
import Combine
import CCStatusShared

@MainActor
final class StatusBarController {
    private let statusItem: NSStatusItem
    private let sessionStore: SessionStore
    private var cancellables = Set<AnyCancellable>()

    init(sessionStore: SessionStore) {
        self.sessionStore = sessionStore
        self.statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)

        setupButton()
        observeChanges()
    }

    private func setupButton() {
        guard let button = statusItem.button else { return }
        button.title = "○"
        button.target = self
        button.action = #selector(statusBarClicked)
    }

    private func observeChanges() {
        sessionStore.$sessions
            .receive(on: DispatchQueue.main)
            .sink { [weak self] _ in
                self?.updateIcon()
            }
            .store(in: &cancellables)
    }

    private func updateIcon() {
        guard let button = statusItem.button else { return }

        let waiting = sessionStore.waitingCount
        let done = sessionStore.doneCount
        let total = sessionStore.needsAttentionCount

        if sessionStore.sessions.isEmpty {
            button.title = "○"
            button.contentTintColor = .secondaryLabelColor
        } else if waiting > 0 {
            button.title = "● \(total)"
            button.contentTintColor = .systemOrange
        } else if done > 0 {
            button.title = "● \(total)"
            button.contentTintColor = .systemGreen
        } else {
            button.title = "●"
            button.contentTintColor = .secondaryLabelColor
        }
    }

    @objc private func statusBarClicked() {
        buildMenu()
    }

    private func buildMenu() {
        let menu = NSMenu()

        let header = NSMenuItem(title: "CC Status", action: nil, keyEquivalent: "")
        header.isEnabled = false
        menu.addItem(header)
        menu.addItem(NSMenuItem.separator())

        let sorted = sessionStore.sortedSessions

        if sorted.isEmpty {
            let empty = NSMenuItem(title: "No active sessions", action: nil, keyEquivalent: "")
            empty.isEnabled = false
            menu.addItem(empty)
        } else {
            for session in sorted {
                let icon: String
                switch session.status {
                case .waiting: icon = "🟠"
                case .done: icon = "🟢"
                case .active: icon = "⚫"
                }

                let name = sessionStore.displayName(for: session)
                let titleItem = NSMenuItem(
                    title: "\(icon) \(name)",
                    action: #selector(sessionClicked(_:)),
                    keyEquivalent: ""
                )
                titleItem.target = self
                titleItem.representedObject = session.sessionId
                menu.addItem(titleItem)

                if !session.summary.isEmpty {
                    let summaryItem = NSMenuItem(title: "    \(session.summary)", action: nil, keyEquivalent: "")
                    summaryItem.isEnabled = false
                    let font = NSFont.systemFont(ofSize: NSFont.smallSystemFontSize)
                    summaryItem.attributedTitle = NSAttributedString(
                        string: "    \(session.summary)",
                        attributes: [
                            .font: font,
                            .foregroundColor: NSColor.secondaryLabelColor
                        ]
                    )
                    menu.addItem(summaryItem)
                }
            }
        }

        if sessionStore.doneCount > 0 {
            menu.addItem(NSMenuItem.separator())
            let dismiss = NSMenuItem(
                title: "Dismiss All Done",
                action: #selector(dismissAllDone),
                keyEquivalent: ""
            )
            dismiss.target = self
            menu.addItem(dismiss)
        }

        menu.addItem(NSMenuItem.separator())
        let quit = NSMenuItem(title: "Quit", action: #selector(quitApp), keyEquivalent: "q")
        quit.target = self
        menu.addItem(quit)

        statusItem.menu = menu
        statusItem.button?.performClick(nil)
        // Reset menu so click action works next time
        DispatchQueue.main.async { [weak self] in
            self?.statusItem.menu = nil
        }
    }

    @objc private func sessionClicked(_ sender: NSMenuItem) {
        guard let sessionId = sender.representedObject as? String,
              let session = sessionStore.sessions[sessionId],
              let terminalId = session.terminalId else { return }

        TerminalJumper.focusTerminal(terminalId: terminalId)
    }

    @objc private func dismissAllDone() {
        sessionStore.dismissDone()
    }

    @objc private func quitApp() {
        NSApp.terminate(nil)
    }
}
