// Run only against a synthetic Scanner qualification process. No permission prompts/bypasses.
import AppKit
import ApplicationServices
import Foundation

func fail(_ message: String) -> Never {
    FileHandle.standardError.write(Data((message + "\n").utf8))
    exit(2)
}

guard CommandLine.arguments.count >= 3,
      let pid = Int32(CommandLine.arguments[1]) else {
    fail("usage: swift platform-macos.swift PID OUTPUT_DIRECTORY [tab,space,escape,...]")
}
let output = URL(fileURLWithPath: CommandLine.arguments[2], isDirectory: true)
try FileManager.default.createDirectory(at: output, withIntermediateDirectories: true)
let application = AXUIElementCreateApplication(pid)
let windows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
let owned = windows.filter { $0[kCGWindowOwnerPID as String] as? Int32 == pid && $0[kCGWindowLayer as String] as? Int == 0 }
let targetTitle = CommandLine.arguments.count > 4 ? CommandLine.arguments[4] : "Scanner"
guard ["Scanner", "Settings"].contains(targetTitle) else { fail("unsupported synthetic window title") }
let targetWindows = owned.filter { $0[kCGWindowName as String] as? String == targetTitle }
func value(_ element: AXUIElement, _ attribute: String) -> CFTypeRef? {
    var result: CFTypeRef?
    return AXUIElementCopyAttributeValue(element, attribute as CFString, &result) == .success ? result : nil
}
func inspect(_ element: AXUIElement, _ depth: Int = 0) -> [String: Any] {
    var record: [String: Any] = [:]
    for attribute in [kAXRoleAttribute, kAXTitleAttribute, kAXDescriptionAttribute, kAXValueAttribute, kAXFocusedAttribute, kAXEnabledAttribute, kAXSelectedAttribute] {
        if let result = value(element, attribute) {
            record[attribute] = String(describing: result)
        }
    }
    if depth < 12, let children = value(element, kAXChildrenAttribute) as? [AXUIElement] {
        record["children"] = children.map { inspect($0, depth + 1) }
    }
    return record
}
guard AXIsProcessTrusted() else { fail("Accessibility permission unavailable; qualification pending") }
func inspectWindows() -> [String: Any] {
    let appWindows = value(application, kAXWindowsAttribute) as? [AXUIElement] ?? []
    return ["AXRole": "AXApplication", "children": appWindows.map { inspect($0) }]
}
let before = inspectWindows()
try JSONSerialization.data(withJSONObject: before, options: [.prettyPrinted, .sortedKeys])
    .write(to: output.appendingPathComponent("accessibility-before.json"))
if CommandLine.arguments.count > 3 {
    let codes: [String: CGKeyCode] = ["b": 11, "o": 31, "s": 1, "p": 35, "r": 15, "k": 40, "l": 37, "comma": 43, "a": 0, "tab": 48, "space": 49, "escape": 53, "return": 36,
                                    "left": 123, "right": 124, "down": 125, "up": 126]
    NSRunningApplication(processIdentifier: pid)?.activate(options: [.activateAllWindows])
    _ = AXUIElementSetAttributeValue(application, kAXFrontmostAttribute as CFString, kCFBooleanTrue)
    if let appWindows = value(application, kAXWindowsAttribute) as? [AXUIElement],
       let target = appWindows.first(where: { value($0, kAXTitleAttribute) as? String == targetTitle }) {
        _ = AXUIElementSetAttributeValue(target, kAXMinimizedAttribute as CFString, kCFBooleanFalse)
        _ = AXUIElementPerformAction(target, kAXRaiseAction as CFString)
    }
    Thread.sleep(forTimeInterval: 0.25)
    let separator: Character = CommandLine.arguments[3].contains(";") ? ";" : ","
    for key in CommandLine.arguments[3].split(separator: separator) {
        guard NSWorkspace.shared.frontmostApplication?.processIdentifier == pid else { fail("synthetic app lost focus; no input sent") }
        if key.hasPrefix("axpress:") || key.hasPrefix("axfocus:") || key.hasPrefix("axvalue|") {
            let parts = key.split(separator: "|", omittingEmptySubsequences: false).map(String.init)
            let title = key.hasPrefix("axvalue|") && parts.count == 3 ? parts[1] : String(key.dropFirst(8))
            func find(_ e: AXUIElement) -> AXUIElement? {
                if value(e, kAXTitleAttribute) as? String == title { return e }
                for child in value(e, kAXChildrenAttribute) as? [AXUIElement] ?? [] {
                    if let result = find(child) { return result }
                }
                return nil
            }
            guard let control = (value(application, kAXWindowsAttribute) as? [AXUIElement] ?? []).compactMap({ find($0) }).first else { fail("native accessible control not found") }
            let result: AXError
            if key.hasPrefix("axfocus:") {
                result = AXUIElementSetAttributeValue(control, kAXFocusedAttribute as CFString, kCFBooleanTrue)
            } else if key.hasPrefix("axvalue|"), parts.count == 3 {
                result = AXUIElementSetAttributeValue(control, kAXValueAttribute as CFString, parts[2] as CFString)
            } else {
                result = AXUIElementPerformAction(control, kAXPressAction as CFString)
            }
            guard result == .success else { fail("native accessible control action failed") }
            Thread.sleep(forTimeInterval: 0.5)
            continue
        }
        if key.hasPrefix("click:") {
            let parts = key.split(separator: ":")
            guard parts.count == 3, let dx = Double(parts[1]), let dy = Double(parts[2]),
                  let bounds = targetWindows.first?[kCGWindowBounds as String] as? [String: Double],
                  let x = bounds["X"], let y = bounds["Y"], let width = bounds["Width"], let height = bounds["Height"],
                  dx >= 0, dy >= 0, dx < width, dy < height else { fail("click must be inside the synthetic window") }
            let point = CGPoint(x: x + dx, y: y + dy)
            CGEvent(mouseEventSource: nil, mouseType: .mouseMoved, mouseCursorPosition: point, mouseButton: .left)?.post(tap: .cghidEventTap)
            Thread.sleep(forTimeInterval: 0.1)
            CGEvent(mouseEventSource: nil, mouseType: .leftMouseDown, mouseCursorPosition: point, mouseButton: .left)?.post(tap: .cghidEventTap)
            Thread.sleep(forTimeInterval: 0.1)
            CGEvent(mouseEventSource: nil, mouseType: .leftMouseUp, mouseCursorPosition: point, mouseButton: .left)?.post(tap: .cghidEventTap)
            Thread.sleep(forTimeInterval: 1)
            continue
        }
        let components = key.split(separator: "+").map(String.init)
        guard let last = components.last, let code = codes[last],
              components.dropLast().allSatisfy({ ["cmd", "shift"].contains($0) }) else { fail("unsupported key") }
        var flags: CGEventFlags = []
        if components.contains("cmd") { flags.insert(.maskCommand) }
        if components.contains("shift") { flags.insert(.maskShift) }
        // Native HID delivery is required by GLFW; AX frontmost selects this app.
        guard NSWorkspace.shared.frontmostApplication?.processIdentifier == pid else { fail("synthetic app lost focus; no input sent") }
        for down in [true, false] {
            let event = CGEvent(keyboardEventSource: nil, virtualKey: code, keyDown: down)
            event?.flags = flags
            event?.post(tap: .cghidEventTap)
        }
        Thread.sleep(forTimeInterval: 0.25)
    }
}
try JSONSerialization.data(withJSONObject: inspectWindows(), options: [.prettyPrinted, .sortedKeys])
    .write(to: output.appendingPathComponent("accessibility-after.json"))
// Wait for this owned app's Stage Manager transition; never capture a thumbnail.
var currentWindows: [[String: Any]] = []
var visible = false
for _ in 0..<5 {
    NSRunningApplication(processIdentifier: pid)?.activate(options: [.activateAllWindows])
    _ = AXUIElementSetAttributeValue(application, kAXFrontmostAttribute as CFString, kCFBooleanTrue)
    RunLoop.current.run(until: Date().addingTimeInterval(0.75))
    currentWindows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
    visible = NSWorkspace.shared.frontmostApplication?.processIdentifier == pid && currentWindows.contains { window in
        guard window[kCGWindowOwnerPID as String] as? Int32 == pid,
              window[kCGWindowLayer as String] as? Int == 0,
              let b = window[kCGWindowBounds as String] as? [String: Double] else { return false }
        return (b["Width"] ?? 0) >= 300 && (b["Height"] ?? 0) >= 200
    }
    if visible { break }
}
guard visible else { fail("synthetic app is not foreground/full-sized; no screenshot captured") }
var captures = 0
for window in currentWindows where window[kCGWindowOwnerPID as String] as? Int32 == pid {
    guard let id = window[kCGWindowNumber as String] as? UInt32,
          window[kCGWindowLayer as String] as? Int == 0 else { continue }
    guard let bounds = window[kCGWindowBounds as String] as? [String: Double],
          (bounds["Width"] ?? 0) >= 300, (bounds["Height"] ?? 0) >= 200 else {
        fail("synthetic window is a thumbnail; visible-focus evidence pending")
    }
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/sbin/screencapture")
    process.arguments = ["-x", "-l", String(id), output.appendingPathComponent("window-\(id).png").path]
    try process.run()
    process.waitUntilExit()
    if process.terminationStatus == 0 { captures += 1 }
}
print("Captured \(captures) application windows; inspect images and accessibility evidence manually.")
if captures == 0 { exit(2) }
