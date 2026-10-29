// Claude Notify helper: posts one macOS notification with this app's icon and,
// optionally, waits for the user to answer it from the notification itself.
//
// Usage: "Claude Notify" --title T [--body B] [--sound NAME|none]
//                        [--actions permission|reply] [--wait SECONDS] [--id ID] [--parent PID]
//
// With --wait, stays alive until answered or timed out. The answer is written to
// ~/Library/Caches/claude-notify/responses/<id>.json:
//   {"action":"allow"}  {"action":"deny"}  {"action":"reply","text":"..."}
//   {"action":"open"}   (notification clicked)
//   {"action":"error"}  (could not post it; `open -W` hides the exit code)
// With --parent, gives up as soon as that process is gone.
// Launch it with `open -n -W -a` so macOS registers it as the app and routes clicks to it.
//
// Runs as a real (accessory) AppKit app: usernoted only grants notification
// permission to processes that check in with the window server as an app.

import AppKit
import UserNotifications

var args: [String: String] = [:]
var it = CommandLine.arguments.dropFirst().makeIterator()
while let key = it.next() {
    guard key.hasPrefix("--"), let value = it.next() else { continue }
    args[String(key.dropFirst(2))] = value
}

// Answers are routed through files: with several helpers running, macOS may hand the
// click to any instance (or relaunch the app), so whoever gets it writes it down.
let responsesDir = FileManager.default.homeDirectoryForCurrentUser
    .appendingPathComponent("Library/Caches/claude-notify/responses")

// Errors also go to a log file, since a LaunchServices launch has no visible stderr
func logError(_ text: String) {
    FileHandle.standardError.write("\(text)\n".data(using: .utf8)!)
    let url = FileManager.default.homeDirectoryForCurrentUser
        .appendingPathComponent("Library/Logs/claude-notify-helper.log")
    let line = "\(Date()) \(text)\n".data(using: .utf8)!
    if let handle = try? FileHandle(forWritingTo: url) {
        handle.seekToEndOfFile()
        handle.write(line)
        try? handle.close()
    } else {
        try? line.write(to: url)
    }
}

func registerCategories(_ center: UNUserNotificationCenter) {
    let allow = UNNotificationAction(identifier: "allow", title: "Allow", options: [])
    let deny = UNNotificationAction(identifier: "deny", title: "Deny", options: [.destructive])
    let reply = UNTextInputNotificationAction(identifier: "reply", title: "Reply", options: [],
                                              textInputButtonTitle: "Send", textInputPlaceholder: "Message Claude…")
    center.setNotificationCategories([
        UNNotificationCategory(identifier: "permission", actions: [allow, deny], intentIdentifiers: []),
        UNNotificationCategory(identifier: "reply", actions: [reply], intentIdentifiers: []),
    ])
}

final class Delegate: NSObject, NSApplicationDelegate, UNUserNotificationCenterDelegate {
    let id = args["id"] ?? UUID().uuidString
    let wait = Double(args["wait"] ?? "") ?? 0
    let parent = pid_t(args["parent"] ?? "") ?? 0
    var signalSources: [DispatchSourceSignal] = []

    func applicationDidFinishLaunching(_ notification: Notification) {
        let center = UNUserNotificationCenter.current()
        center.delegate = self
        registerCategories(center)

        // Relaunched by macOS only to deliver a click: didReceive handles it
        guard args["title"] != nil else {
            DispatchQueue.main.asyncAfter(deadline: .now() + 5) { exit(0) }
            return
        }

        center.requestAuthorization(options: [.alert, .sound]) { granted, error in
            guard granted else {
                logError("notifications not allowed: \(error?.localizedDescription ?? "denied") [bundle=\(Bundle.main.bundleIdentifier ?? "nil")]")
                self.finish(2)
                return
            }
            self.post(center)
        }
    }

    func post(_ center: UNUserNotificationCenter) {
        let content = UNMutableNotificationContent()
        content.title = args["title"] ?? "Claude"
        if let body = args["body"] { content.body = body }
        if let actions = args["actions"] { content.categoryIdentifier = actions }
        // Only a waiting instance reads the answer, so a click on a plain notification writes none
        if wait > 0 { content.userInfo = ["id": id] }

        switch args["sound"] {
        case "none": break
        case let name?: content.sound = UNNotificationSound(named: UNNotificationSoundName(name))
        case nil: content.sound = .default
        }

        let request = UNNotificationRequest(identifier: id, content: content, trigger: nil)
        center.add(request) { error in
            if let error {
                logError("post failed: \(error.localizedDescription)")
                self.finish(1)
            } else if self.wait > 0 {
                DispatchQueue.main.async { self.waitForAnswer() }
            } else {
                // Give usernoted a moment to take the notification before we quit
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) { self.finish(0) }
            }
        }
    }

    func waitForAnswer() {
        // Killed because the answer is no longer wanted: take the notification away
        for sig in [SIGTERM, SIGINT, SIGHUP] {
            signal(sig, SIG_IGN)
            let source = DispatchSource.makeSignalSource(signal: sig, queue: .main)
            source.setEventHandler { self.withdraw(); self.finish(0) }
            source.resume()
            signalSources.append(source)
        }

        // The caller launched us through LaunchServices (so macOS can route the click
        // back to this instance): it finds us by this pid file and reads the answer file.
        try? FileManager.default.createDirectory(at: responsesDir, withIntermediateDirectories: true)
        let pidFile = responsesDir.appendingPathComponent("\(id).pid")
        try? String(getpid()).write(to: pidFile, atomically: true, encoding: .utf8)

        let file = responsesDir.appendingPathComponent("\(id).json")
        let deadline = Date().addingTimeInterval(wait)
        Timer.scheduledTimer(withTimeInterval: 0.3, repeats: true) { timer in
            let answered = FileManager.default.fileExists(atPath: file.path)
            // The hook may be killed before it can read our pid file and stop us
            let orphaned = self.parent > 0 && kill(self.parent, 0) != 0
            if answered || orphaned || Date() > deadline {
                timer.invalidate()
                self.withdraw()
                self.finish(0)
            }
        }
    }

    func withdraw() {
        UNUserNotificationCenter.current().removeDeliveredNotifications(withIdentifiers: [id])
    }

    // Show the banner even though this app is technically running when it fires
    func userNotificationCenter(_ center: UNUserNotificationCenter, willPresent notification: UNNotification,
                                withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void) {
        completionHandler([.banner, .list, .sound])
    }

    func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse,
                                withCompletionHandler completionHandler: @escaping () -> Void) {
        defer { completionHandler() }
        guard let target = response.notification.request.content.userInfo["id"] as? String else { return }

        var answer: [String: String]
        switch response.actionIdentifier {
        case "allow": answer = ["action": "allow"]
        case "deny": answer = ["action": "deny"]
        case "reply":
            let text = (response as? UNTextInputNotificationResponse)?.userText ?? ""
            answer = ["action": "reply", "text": text]
        case UNNotificationDefaultActionIdentifier: answer = ["action": "open"]
        default: return  // dismissed
        }

        try? FileManager.default.createDirectory(at: responsesDir, withIntermediateDirectories: true)
        if let data = try? JSONSerialization.data(withJSONObject: answer) {
            try? data.write(to: responsesDir.appendingPathComponent("\(target).json"), options: .atomic)
        }
    }

    func finish(_ code: Int32) {
        try? FileManager.default.removeItem(at: responsesDir.appendingPathComponent("\(id).pid"))
        if code != 0 && wait > 0 {
            try? FileManager.default.createDirectory(at: responsesDir, withIntermediateDirectories: true)
            try? Data(#"{"action":"error"}"#.utf8).write(to: responsesDir.appendingPathComponent("\(id).json"), options: .atomic)
        }
        DispatchQueue.main.async { exit(code) }
    }
}

let app = NSApplication.shared
let delegate = Delegate()
app.delegate = delegate
app.setActivationPolicy(.accessory)
app.run()
