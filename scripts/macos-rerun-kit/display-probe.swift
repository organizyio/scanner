import AppKit
import ApplicationServices
import CoreGraphics
import Foundation

func save(_ data: [String: Any], _ path: String) throws {
    try FileManager.default.createDirectory(at: URL(fileURLWithPath: path).deletingLastPathComponent(), withIntermediateDirectories: true)
    try JSONSerialization.data(withJSONObject: data, options: [.prettyPrinted, .sortedKeys])
        .write(to: URL(fileURLWithPath: path), options: .atomic)
}
func attr(_ e: AXUIElement, _ name: String) -> CFTypeRef? {
    var v: CFTypeRef?
    return AXUIElementCopyAttributeValue(e, name as CFString, &v) == .success ? v : nil
}
func rectData(_ r: CGRect) -> [String: Double] {
    return ["x": r.origin.x, "y": r.origin.y, "width": r.width, "height": r.height]
}
func screens() -> [[String: Any]] {
    return NSScreen.screens.compactMap { screen in
        guard let number = screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? NSNumber else { return nil }
        let id = CGDirectDisplayID(number.uint32Value)
        return ["display_id": id, "backing_scale": Double(screen.backingScaleFactor),
                "quartz_bounds": rectData(CGDisplayBounds(id)), "appkit_frame": rectData(screen.frame),
                "pixels_wide": CGDisplayPixelsWide(id), "pixels_high": CGDisplayPixelsHigh(id)]
    }
}
let args = CommandLine.arguments
if args.count == 3 && args[1] == "--host" {
    try save(["os": ProcessInfo.processInfo.operatingSystemVersionString, "screens": screens()], args[2])
    exit(0)
}
if args.count >= 3 && args[1] == "--permissions" {
    if args.count == 4 && args[3] == "--request" {
        let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: true] as CFDictionary
        _ = AXIsProcessTrustedWithOptions(options)
        _ = CGRequestScreenCaptureAccess()
    }
    let ax = AXIsProcessTrusted(), capture = CGPreflightScreenCaptureAccess()
    try save(["accessibility_authorized": ax, "screen_capture_authorized": capture], args[2])
    exit(ax && capture ? 0 : 2)
}
guard args.count >= 4, let pid = Int32(args[1]) else {
    fputs("usage: display-probe PID OUTPUT WINDOW_TITLE [CONTROL_TITLE] [EXPECTED_SCALE]\n", stderr)
    exit(2)
}
let output = args[2], title = args[3]
guard ["Scanner", "Settings"].contains(title) else { exit(2) }
let windows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
guard let window = windows.first(where: {
    $0[kCGWindowOwnerPID as String] as? Int32 == pid && $0[kCGWindowName as String] as? String == title &&
    $0[kCGWindowLayer as String] as? Int == 0
}), let values = window[kCGWindowBounds as String] as? [String: Double],
   let x = values["X"], let y = values["Y"], let w = values["Width"], let h = values["Height"], w >= 300, h >= 200 else {
    try save(["passed": false, "reason": "Owned full-sized visible window not found", "pid": pid], output)
    exit(3)
}
let bounds = CGRect(x: x, y: y, width: w, height: h)
var candidates: [[String: Any]] = []
for screen in NSScreen.screens {
    guard let number = screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? NSNumber else { continue }
    let id = CGDirectDisplayID(number.uint32Value)
    let intersection = bounds.intersection(CGDisplayBounds(id))
    let fraction: Double = intersection.isNull ? 0 : Double(intersection.width * intersection.height / (bounds.width * bounds.height))
    candidates.append(["display_id": id, "backing_scale": Double(screen.backingScaleFactor), "window_fraction": fraction])
}
candidates.sort { ($0["window_fraction"] as! Double) > ($1["window_fraction"] as! Double) }
guard let best = candidates.first else { exit(3) }
let expected = args.count > 5 ? Double(args[5]) : nil
let actualScale = best["backing_scale"] as! Double
var passed = (best["window_fraction"] as! Double) >= 0.98 && (expected == nil || abs(actualScale - expected!) < 0.01)
var result: [String: Any] = ["passed": passed, "pid": pid, "title": title,
    "window_id": window[kCGWindowNumber as String] ?? 0, "window_bounds": rectData(bounds),
    "actual_backing_scale": actualScale, "display_candidates": candidates, "screens": screens(),
    "foreground_owned": NSWorkspace.shared.frontmostApplication?.processIdentifier == pid]
if let expected = expected { result["expected_scale"] = expected }
if args.count > 4 && !args[4].isEmpty {
    let app = AXUIElementCreateApplication(pid)
    func find(_ e: AXUIElement, _ name: String) -> AXUIElement? {
        if attr(e, kAXTitleAttribute) as? String == name { return e }
        for child in attr(e, kAXChildrenAttribute) as? [AXUIElement] ?? [] {
            if let found = find(child, name) { return found }
        }
        return nil
    }
    let ownedAXWindow = (attr(app, kAXWindowsAttribute) as? [AXUIElement] ?? []).first {
        attr($0, kAXTitleAttribute) as? String == title
    }
    if let ownedAXWindow = ownedAXWindow, let control = find(ownedAXWindow, args[4]),
       let p = attr(control, kAXPositionAttribute), let z = attr(control, kAXSizeAttribute),
       CFGetTypeID(p) == AXValueGetTypeID(), CFGetTypeID(z) == AXValueGetTypeID() {
        var point = CGPoint.zero, size = CGSize.zero
        if AXValueGetValue(p as! AXValue, .cgPoint, &point) && AXValueGetValue(z as! AXValue, .cgSize, &size) {
            let center = CGPoint(x: point.x + size.width / 2, y: point.y + size.height / 2)
            var hit: AXUIElement?
            let hitOK = AXUIElementCopyElementAtPosition(app, Float(center.x), Float(center.y), &hit) == .success &&
                hit != nil && attr(hit!, kAXTitleAttribute) as? String == args[4]
            let inside = bounds.contains(center) && size.width > 0 && size.height > 0
            result["control"] = ["title": args[4], "ax_bounds": rectData(CGRect(origin: point, size: size)),
                "center_in_window": inside, "ax_center_hit_matches": hitOK,
                "client_x": center.x - bounds.origin.x, "client_y": center.y - bounds.origin.y]
            passed = passed && inside && hitOK
        } else { passed = false }
    } else { passed = false; result["control_error"] = "Native control/bounds unavailable" }
}
result["passed"] = passed
try save(result, output)
exit(passed ? 0 : 4)
