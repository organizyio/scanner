// Observe only the owned synthetic process; never change accessibility settings.
import AppKit
import ApplicationServices
import Foundation

guard CommandLine.arguments.count == 3, let pid = Int32(CommandLine.arguments[1]),
      AXIsProcessTrusted() else { exit(2) }
let output = URL(fileURLWithPath: CommandLine.arguments[2], isDirectory: true)
try FileManager.default.createDirectory(at: output, withIntermediateDirectories: true)
let application = AXUIElementCreateApplication(pid)
AXUIElementSetMessagingTimeout(application, 1)
var events: [String] = []
var observer: AXObserver?
guard AXObserverCreate(pid, { _, _, notification, _ in
    events.append(notification as String)
}, &observer) == .success, let observer = observer else { exit(3) }
let names = [kAXFocusedUIElementChangedNotification, kAXWindowCreatedNotification]
let registered = names.filter {
    AXObserverAddNotification(observer, application, $0 as CFString, nil) == .success
}
guard !registered.isEmpty else { exit(4) }
let source = AXObserverGetRunLoopSource(observer)
CFRunLoopAddSource(CFRunLoopGetCurrent(), source, .defaultMode)
func save(_ name: String, _ value: [String: Any]) throws {
    try JSONSerialization.data(withJSONObject: value, options: [.prettyPrinted, .sortedKeys])
        .write(to: output.appendingPathComponent(name), options: .atomic)
}
try save("ready.json", ["pid": pid, "notifications": registered])
let deadline = Date().addingTimeInterval(180)
while Date() < deadline,
      !FileManager.default.fileExists(atPath: output.appendingPathComponent("detach").path),
      NSRunningApplication(processIdentifier: pid)?.isTerminated == false {
    RunLoop.current.run(until: Date().addingTimeInterval(0.05))
}
for name in registered {
    AXObserverRemoveNotification(observer, application, name as CFString)
}
CFRunLoopRemoveSource(CFRunLoopGetCurrent(), source, .defaultMode)
try save("detached.json", ["pid": pid, "events": events,
    "requested": FileManager.default.fileExists(atPath: output.appendingPathComponent("detach").path),
    "process_exited": NSRunningApplication(processIdentifier: pid)?.isTerminated != false])
// An expired observer is a failure, not a successful detach.
exit(Date() < deadline ? 0 : 5)
