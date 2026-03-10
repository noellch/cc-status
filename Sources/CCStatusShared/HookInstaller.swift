import Foundation

public enum HookInstaller {

    // MARK: - Public

    /// The hook events we manage.
    private static let hookEvents = [
        "SessionStart",
        "UserPromptSubmit",
        "PreToolUse",
        "PostToolUse",
        "Stop",
        "Notification",
        "SessionEnd",
    ]

    /// Build our hook entry that gets inserted into each event's hooks array.
    /// Uses the full path of the currently running binary so hooks work regardless of $PATH.
    private static func makeOurHookEntry() -> [String: Any] {
        let hookPath = resolveHookPath()
        return [
            "type": "command",
            "command": hookPath,
            "async": true,
            "timeout": 5,
        ]
    }

    /// Resolve the full path to cc-status-hook binary.
    private static func resolveHookPath() -> String {
        let currentExe = CommandLine.arguments[0]

        // If argv[0] is already an absolute or relative path, use it directly
        if currentExe.contains("/") {
            let url = URL(fileURLWithPath: currentExe).standardized
            // Resolve symlinks so the path survives brew re-installs
            let resolved = (try? URL(fileURLWithPath: url.path).resolvingSymlinksInPath()) ?? url
            return resolved.path
        }

        // argv[0] has no path separator — binary was found via $PATH.
        // Use `which` to resolve the full path.
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/which")
        process.arguments = [currentExe]
        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = FileHandle.nullDevice
        do {
            try process.run()
            process.waitUntilExit()
            let data = pipe.fileHandleForReading.readDataToEndOfFile()
            let path = String(data: data, encoding: .utf8)?
                .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            if !path.isEmpty {
                let url = URL(fileURLWithPath: path)
                let resolved = (try? url.resolvingSymlinksInPath()) ?? url
                return resolved.path
            }
        } catch {}

        // Last resort: assume it's in /opt/homebrew/bin or /usr/local/bin
        let fallbacks = ["/opt/homebrew/bin/\(currentExe)", "/usr/local/bin/\(currentExe)"]
        for fallback in fallbacks {
            if FileManager.default.isExecutableFile(atPath: fallback) {
                return fallback
            }
        }

        return currentExe
    }

    /// The marker we use to detect our own hook entries.
    private static let commandMarker = "cc-status-hook"

    // MARK: - Install

    public static func install() throws {
        let settingsURL = settingsFileURL()
        var root = try readOrCreateSettings(at: settingsURL)

        var hooks = root["hooks"] as? [String: Any] ?? [:]
        let ourEntry = makeOurHookEntry()
        let ourCommand = ourEntry["command"] as! String
        var added: [String] = []
        var updated: [String] = []

        for event in hookEvents {
            var matcherGroups = hooks[event] as? [[String: Any]] ?? []

            if let (groupIdx, hookIdx) = findOurHook(in: matcherGroups) {
                // Already exists — check if path needs updating
                if var group = matcherGroups[groupIdx] as? [String: Any],
                   var groupHooks = group["hooks"] as? [[String: Any]] {
                    let existing = groupHooks[hookIdx]["command"] as? String ?? ""
                    if existing != ourCommand {
                        groupHooks[hookIdx] = ourEntry
                        group["hooks"] = groupHooks
                        matcherGroups[groupIdx] = group
                        hooks[event] = matcherGroups
                        updated.append(event)
                    }
                }
                continue
            }

            // Not found — append new matcher group
            let newGroup: [String: Any] = [
                "hooks": [ourEntry]
            ]
            matcherGroups.append(newGroup)
            hooks[event] = matcherGroups
            added.append(event)
        }

        root["hooks"] = hooks
        try writeSettings(root, to: settingsURL)

        if added.isEmpty && updated.isEmpty {
            print("cc-status hooks already installed in ~/.claude/settings.json")
        } else {
            if !added.isEmpty {
                print("Installed cc-status hooks for events: \(added.joined(separator: ", "))")
            }
            if !updated.isEmpty {
                print("Updated cc-status hook path for events: \(updated.joined(separator: ", "))")
            }
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
    /// Uses flock to prevent concurrent writes from multiple hook processes.
    private static func writeSettings(_ dict: [String: Any], to url: URL) throws {
        let data = try JSONSerialization.data(
            withJSONObject: dict,
            options: [.prettyPrinted, .sortedKeys]
        )

        let lockPath = url.path + ".lock"
        let lockFD = open(lockPath, O_CREAT | O_WRONLY, 0o644)
        guard lockFD >= 0 else {
            // Fallback: write without lock
            try data.write(to: url, options: .atomic)
            return
        }
        defer {
            flock(lockFD, LOCK_UN)
            close(lockFD)
        }

        guard flock(lockFD, LOCK_EX) == 0 else {
            // Fallback: write without lock
            try data.write(to: url, options: .atomic)
            return
        }

        try data.write(to: url, options: .atomic)
    }

    /// Find our hook entry and return (matcherGroupIndex, hookIndex), or nil.
    private static func findOurHook(in matcherGroups: [[String: Any]]) -> (Int, Int)? {
        for (gi, group) in matcherGroups.enumerated() {
            if let groupHooks = group["hooks"] as? [[String: Any]] {
                for (hi, hook) in groupHooks.enumerated() {
                    if isOurHookEntry(hook) {
                        return (gi, hi)
                    }
                }
            }
        }
        return nil
    }

    /// Check if a hook entry is ours by looking for our command marker.
    private static func isOurHookEntry(_ hook: [String: Any]) -> Bool {
        guard let command = hook["command"] as? String else { return false }
        return command.contains(commandMarker)
    }
}
