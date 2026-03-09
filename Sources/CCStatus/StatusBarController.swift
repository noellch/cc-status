import AppKit
import Combine
import CCStatusShared
import ServiceManagement

// MARK: - Palette (muted, warm tones)

private enum Palette {
    static let waiting = NSColor(red: 0.91, green: 0.61, blue: 0.30, alpha: 1)  // warm amber
    static let done    = NSColor(red: 0.48, green: 0.69, blue: 0.43, alpha: 1)  // sage green
    static let active  = NSColor(red: 0.42, green: 0.56, blue: 0.68, alpha: 1)  // slate blue
    static let idle    = NSColor.tertiaryLabelColor
    static let sub     = NSColor.secondaryLabelColor
}

@MainActor
final class StatusBarController: NSObject, NSMenuDelegate {
    private let statusItem: NSStatusItem
    private let sessionStore: SessionStore
    private var cancellables = Set<AnyCancellable>()

    init(sessionStore: SessionStore) {
        self.sessionStore = sessionStore
        self.statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        super.init()

        setupButton()
        observeChanges()
    }

    private func setupButton() {
        guard let button = statusItem.button else { return }
        button.attributedTitle = NSAttributedString(
            string: "○",
            attributes: [.foregroundColor: Palette.idle, .font: NSFont.systemFont(ofSize: 13, weight: .light)]
        )

        let menu = NSMenu()
        menu.delegate = self
        menu.autoenablesItems = false
        statusItem.menu = menu
    }

    private func observeChanges() {
        sessionStore.$sessions
            .receive(on: DispatchQueue.main)
            .sink { [weak self] _ in
                self?.updateIcon()
            }
            .store(in: &cancellables)
    }

    // MARK: - Menu Bar Icon

    private func updateIcon() {
        guard let button = statusItem.button else { return }

        if sessionStore.sessions.isEmpty {
            button.attributedTitle = NSAttributedString(
                string: "○",
                attributes: [.foregroundColor: Palette.idle, .font: NSFont.systemFont(ofSize: 13, weight: .light)]
            )
            return
        }

        let waiting = sessionStore.waitingCount
        let done = sessionStore.doneCount
        let active = sessionStore.sessions.values.filter { $0.status == .active }.count

        let segments: [(Int, NSColor)] = [
            (waiting, Palette.waiting),
            (done, Palette.done),
            (active, Palette.active),
        ].filter { $0.0 > 0 }

        let result = NSMutableAttributedString()
        let dotFont = NSFont.systemFont(ofSize: 11, weight: .regular)
        let numFont = NSFont.monospacedDigitSystemFont(ofSize: 10, weight: .light)

        for (i, segment) in segments.enumerated() {
            let (count, color) = segment
            if i > 0 {
                result.append(NSAttributedString(string: "  ", attributes: [.font: dotFont]))
            }
            result.append(NSAttributedString(
                string: "●",
                attributes: [.foregroundColor: color, .font: dotFont]
            ))
            if count > 1 {
                result.append(NSAttributedString(
                    string: " \(count)",
                    attributes: [.foregroundColor: color.withAlphaComponent(0.8), .font: numFont]
                ))
            }
        }

        button.attributedTitle = result
    }

    // MARK: - NSMenuDelegate

    nonisolated func menuNeedsUpdate(_ menu: NSMenu) {
        // NSMenu is always accessed on the main thread by AppKit.
        // Use nonisolated(unsafe) to bridge NSMenu across the isolation boundary.
        nonisolated(unsafe) let unsafeMenu = menu
        MainActor.assumeIsolated {
            rebuildMenu(unsafeMenu)
        }
    }

    // MARK: - Menu Building

    private static func dotImage(color: NSColor) -> NSImage {
        let s = NSSize(width: 6, height: 6)
        let image = NSImage(size: s, flipped: false) { rect in
            color.setFill()
            NSBezierPath(ovalIn: rect).fill()
            return true
        }
        image.isTemplate = false
        return image
    }

    /// An invisible spacer item to create breathing room between session groups.
    private static func spacerItem() -> NSMenuItem {
        let item = NSMenuItem()
        item.attributedTitle = NSAttributedString(
            string: " ",
            attributes: [.font: NSFont.systemFont(ofSize: 4)]
        )
        item.isEnabled = false
        return item
    }

    private func rebuildMenu(_ menu: NSMenu) {
        menu.removeAllItems()

        let sorted = sessionStore.sortedSessions

        if sorted.isEmpty {
            let empty = NSMenuItem()
            empty.attributedTitle = NSAttributedString(
                string: "—",
                attributes: [.foregroundColor: Palette.idle, .font: NSFont.systemFont(ofSize: 13, weight: .ultraLight)]
            )
            empty.isEnabled = false
            menu.addItem(empty)
        } else {
            for (index, session) in sorted.enumerated() {
                let color: NSColor
                switch session.status {
                case .waiting: color = Palette.waiting
                case .done:    color = Palette.done
                case .active:  color = Palette.active
                case .remove:  continue
                }

                let repo = (session.cwd as NSString).lastPathComponent

                let title = NSMutableAttributedString()
                title.append(NSAttributedString(
                    string: repo,
                    attributes: [.font: NSFont.systemFont(ofSize: 13, weight: .medium)]
                ))
                if !session.branch.isEmpty {
                    title.append(NSAttributedString(
                        string: " · ",
                        attributes: [.foregroundColor: Palette.idle, .font: NSFont.systemFont(ofSize: 12, weight: .light)]
                    ))
                    title.append(NSAttributedString(
                        string: session.branch,
                        attributes: [.foregroundColor: Palette.sub, .font: NSFont.systemFont(ofSize: 12, weight: .light)]
                    ))
                }

                let menuItem = NSMenuItem()
                menuItem.attributedTitle = title
                menuItem.action = #selector(sessionClicked(_:))
                menuItem.target = self
                menuItem.representedObject = session.sessionId
                menuItem.image = Self.dotImage(color: color)
                menuItem.isEnabled = true
                menu.addItem(menuItem)

                if !session.summary.isEmpty {
                    let s = session.summary
                    let text = s.count > 50 ? String(s.prefix(50)) + "…" : s
                    let summaryItem = NSMenuItem()
                    summaryItem.attributedTitle = NSAttributedString(
                        string: text,
                        attributes: [.foregroundColor: NSColor.labelColor.withAlphaComponent(0.55), .font: NSFont.systemFont(ofSize: 11, weight: .light)]
                    )
                    summaryItem.isEnabled = false
                    summaryItem.indentationLevel = 1
                    menu.addItem(summaryItem)
                }

                if index < sorted.count - 1 {
                    menu.addItem(Self.spacerItem())
                }
            }
        }

        // --- Bottom ---
        menu.addItem(NSMenuItem.separator())

        if !sorted.isEmpty {
            let dismiss = NSMenuItem()
            dismiss.attributedTitle = NSAttributedString(
                string: "dismiss all",
                attributes: [.font: NSFont.systemFont(ofSize: 12, weight: .light)]
            )
            dismiss.action = #selector(dismissAll)
            dismiss.target = self
            menu.addItem(dismiss)
        }

        let launchAtLogin = NSMenuItem()
        launchAtLogin.attributedTitle = NSAttributedString(
            string: "launch at login",
            attributes: [.font: NSFont.systemFont(ofSize: 12, weight: .light)]
        )
        launchAtLogin.action = #selector(toggleLaunchAtLogin)
        launchAtLogin.target = self
        launchAtLogin.state = SMAppService.mainApp.status == .enabled ? .on : .off
        menu.addItem(launchAtLogin)

        let quit = NSMenuItem()
        quit.attributedTitle = NSAttributedString(
            string: "quit",
            attributes: [.font: NSFont.systemFont(ofSize: 12, weight: .light)]
        )
        quit.action = #selector(quitApp)
        quit.target = self
        quit.keyEquivalent = "q"
        menu.addItem(quit)
    }

    // MARK: - Actions

    @objc private func sessionClicked(_ sender: NSMenuItem) {
        guard let sessionId = sender.representedObject as? String,
              let session = sessionStore.sessions[sessionId] else { return }

        if let terminalId = session.terminalId {
            TerminalJumper.focusTerminal(terminalId: terminalId)
        } else {
            TerminalJumper.focusAnyTerminal()
        }
    }

    @objc private func dismissAll() {
        sessionStore.dismissAll()
    }

    @objc private func toggleLaunchAtLogin() {
        let service = SMAppService.mainApp
        do {
            if service.status == .enabled {
                try service.unregister()
            } else {
                try service.register()
            }
        } catch {
            print("[CCStatus] Failed to toggle launch at login: \(error)")
        }
    }

    @objc private func quitApp() {
        NSApp.terminate(nil)
    }
}
