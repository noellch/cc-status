import Foundation

public enum HookInstaller {

    // MARK: - Public

    /// The hook events we manage.
    private static let hookEvents = [
        "SessionStart",
        "UserPromptSubmit",
        "Stop",
        "Notification",
        "SessionEnd",
    ]

    /// Build our hook entry that gets inserted into each event's hooks array.
    private static func makeOurHookEntry() -> [String: Any] {
        [
            "type": "command",
            "command": "cc-status-hook",
            "async": true,
            "timeout": 5,
        ]
    }

    /// The marker we use to detect our own hook entries.
    private static let commandMarker = "cc-status-hook"

    // MARK: - Install

    public static func install() throws {
        let settingsURL = settingsFileURL()
        var root = try readOrCreateSettings(at: settingsURL)

        var hooks = root["hooks"] as? [String: Any] ?? [:]
        var added: [String] = []

        for event in hookEvents {
            var matcherGroups = hooks[event] as? [[String: Any]] ?? []

            // Check if we already have our hook in any matcher group
            if containsOurHook(in: matcherGroups) {
                continue
            }

            // Append a new matcher group with our hook
            let newGroup: [String: Any] = [
                "hooks": [makeOurHookEntry()]
            ]
            matcherGroups.append(newGroup)
            hooks[event] = matcherGroups
            added.append(event)
        }

        root["hooks"] = hooks
        try writeSettings(root, to: settingsURL)

        if added.isEmpty {
            print("cc-status hooks already installed in ~/.claude/settings.json")
        } else {
            print("Installed cc-status hooks for events: \(added.joined(separator: ", "))")
            print("Settings written to \(settingsURL.path)")
        }
    }

    // MARK: - Uninstall

    public static func uninstall() throws {
        let settingsURL = settingsFileURL()

        guard FileManager.default.fileExists(atPath: settingsURL.path) else {
            print("No ~/.claude/settings.json found. Nothing to uninstall.")
            return
        }

        let data = try Data(contentsOf: settingsURL)
        guard var root = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            print("Settings file is not a JSON object. Nothing to uninstall.")
            return
        }

        guard var hooks = root["hooks"] as? [String: Any] else {
            print("No hooks section found. Nothing to uninstall.")
            return
        }

        var removed: [String] = []

        for event in hooks.keys.sorted() {
            guard var matcherGroups = hooks[event] as? [[String: Any]] else {
                continue
            }

            var didRemove = false

            // Filter out matcher groups that contain our hook, or remove our hook from groups
            matcherGroups = matcherGroups.compactMap { group -> [String: Any]? in
                guard var groupHooks = group["hooks"] as? [[String: Any]] else {
                    return group
                }

                let before = groupHooks.count
                groupHooks.removeAll { isOurHookEntry($0) }
                if groupHooks.count != before {
                    didRemove = true
                }

                if groupHooks.isEmpty {
                    // The entire matcher group only had our hook — remove it
                    return nil
                }
                var updated = group
                updated["hooks"] = groupHooks
                return updated
            }

            if didRemove {
                removed.append(event)
            }

            if matcherGroups.isEmpty {
                hooks.removeValue(forKey: event)
            } else {
                hooks[event] = matcherGroups
            }
        }

        root["hooks"] = hooks
        try writeSettings(root, to: settingsURL)

        if removed.isEmpty {
            print("No cc-status hooks found in ~/.claude/settings.json. Nothing to uninstall.")
        } else {
            print("Removed cc-status hooks from events: \(removed.joined(separator: ", "))")
            print("Settings written to \(settingsURL.path)")
        }
    }

    // MARK: - Helpers

    private static func settingsFileURL() -> URL {
        FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent(".claude")
            .appendingPathComponent("settings.json")
    }

    /// Read existing settings or create a new file with `{"hooks": {}}`.
    private static func readOrCreateSettings(at url: URL) throws -> [String: Any] {
        let fm = FileManager.default
        let dir = url.deletingLastPathComponent()

        if !fm.fileExists(atPath: dir.path) {
            try fm.createDirectory(at: dir, withIntermediateDirectories: true)
        }

        if fm.fileExists(atPath: url.path) {
            let data = try Data(contentsOf: url)
            if let obj = try JSONSerialization.jsonObject(with: data) as? [String: Any] {
                return obj
            }
        }

        // File doesn't exist or isn't a dict — start fresh
        return ["hooks": [String: Any]()]
    }

    /// Write settings dict back as pretty-printed, sorted-keys JSON.
    private static func writeSettings(_ dict: [String: Any], to url: URL) throws {
        let data = try JSONSerialization.data(
            withJSONObject: dict,
            options: [.prettyPrinted, .sortedKeys]
        )
        try data.write(to: url, options: .atomic)
    }

    /// Check if any matcher group already contains our hook entry.
    private static func containsOurHook(in matcherGroups: [[String: Any]]) -> Bool {
        for group in matcherGroups {
            if let groupHooks = group["hooks"] as? [[String: Any]] {
                for hook in groupHooks {
                    if isOurHookEntry(hook) {
                        return true
                    }
                }
            }
        }
        return false
    }

    /// Check if a hook entry is ours by looking for our command marker.
    private static func isOurHookEntry(_ hook: [String: Any]) -> Bool {
        guard let command = hook["command"] as? String else { return false }
        return command.contains(commandMarker)
    }
}
