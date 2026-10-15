#!/usr/bin/env python3
"""Interact with one synthetic Fyne X11/Xwayland process; never a Wayland bypass.

Requires existing DISPLAY/XAUTHORITY access, libX11/libXtst and Pillow. On native
Wayland windows or unavailable session access, qualification remains pending.
"""
import argparse
import ctypes as C
import ctypes.util
import json
from pathlib import Path
import time


class XImage(C.Structure):
    _fields_ = [("width", C.c_int), ("height", C.c_int), ("xoffset", C.c_int),
                ("format", C.c_int), ("data", C.c_void_p), ("byte_order", C.c_int),
                ("bitmap_unit", C.c_int), ("bitmap_bit_order", C.c_int),
                ("bitmap_pad", C.c_int), ("depth", C.c_int), ("bytes_per_line", C.c_int),
                ("bits_per_pixel", C.c_int)]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pid", type=int, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--keys", default="", help="comma-separated Tab,space,Escape,Return")
    parser.add_argument("--click", help="client-relative x,y points separated by semicolons")
    parser.add_argument("--title", choices=["Scanner", "Settings"], default="Scanner")
    parser.add_argument("--text", default="", help="lowercase ASCII synthetic root path only")
    args = parser.parse_args()
    x = C.CDLL(ctypes.util.find_library("X11") or "libX11.so.6")
    x.XOpenDisplay.argtypes = [C.c_char_p]
    x.XOpenDisplay.restype = C.c_void_p
    display = x.XOpenDisplay(None)
    if not display:
        raise SystemExit("Existing X11 session inaccessible; graphical qualification pending")
    ulong = C.c_ulong
    x.XDefaultRootWindow.argtypes = [C.c_void_p]
    x.XDefaultRootWindow.restype = ulong
    x.XQueryTree.argtypes = [C.c_void_p, ulong, C.POINTER(ulong), C.POINTER(ulong),
                            C.POINTER(C.POINTER(ulong)), C.POINTER(C.c_uint)]
    x.XInternAtom.argtypes = [C.c_void_p, C.c_char_p, C.c_int]
    x.XInternAtom.restype = ulong
    x.XGetWindowProperty.argtypes = [C.c_void_p, ulong, ulong, C.c_long, C.c_long,
                                    C.c_int, ulong, C.POINTER(ulong), C.POINTER(C.c_int),
                                    C.POINTER(ulong), C.POINTER(ulong), C.POINTER(C.c_void_p)]
    x.XFree.argtypes = [C.c_void_p]
    x.XCloseDisplay.argtypes = [C.c_void_p]
    x.XFetchName.argtypes = [C.c_void_p, ulong, C.POINTER(C.c_void_p)]
    atom = x.XInternAtom(display, b"_NET_WM_PID", 0)

    def children(window):
        root, parent, count = ulong(), ulong(), C.c_uint()
        values = C.POINTER(ulong)()
        if not x.XQueryTree(display, window, C.byref(root), C.byref(parent), C.byref(values), C.byref(count)):
            return []
        result = [values[i] for i in range(count.value)]
        if values:
            x.XFree(values)
        return result

    def process_id(window):
        actual, count, remaining = ulong(), ulong(), ulong()
        fmt, data = C.c_int(), C.c_void_p()
        x.XGetWindowProperty(display, window, atom, 0, 1, 0, 0, C.byref(actual),
                             C.byref(fmt), C.byref(count), C.byref(remaining), C.byref(data))
        try:
            return C.cast(data, C.POINTER(ulong))[0] if data and count.value else None
        finally:
            if data:
                x.XFree(data)

    pending = children(x.XDefaultRootWindow(display))
    windows = []
    while pending:
        window = pending.pop()
        if process_id(window) == args.pid:
            title = C.c_void_p()
            x.XFetchName(display, window, C.byref(title))
            try:
                name = C.string_at(title).decode(errors="replace") if title else ""
                if name == args.title:
                    windows.append(window)
            finally:
                if title:
                    x.XFree(title)
        pending.extend(children(window))
    if not windows:
        x.XCloseDisplay(display)
        raise SystemExit("No X11 window for requested process; qualification pending")
    args.output.mkdir(parents=True, exist_ok=True)
    try:
        if args.keys or args.click or args.text:
            xtst = C.CDLL(ctypes.util.find_library("Xtst") or "libXtst.so.6")
            x.XSetInputFocus.argtypes = [C.c_void_p, ulong, C.c_int, ulong]
            x.XStringToKeysym.argtypes = [C.c_char_p]
            x.XStringToKeysym.restype = ulong
            x.XKeysymToKeycode.argtypes = [C.c_void_p, ulong]
            x.XKeysymToKeycode.restype = C.c_uint
            x.XFlush.argtypes = [C.c_void_p]
            xtst.XTestFakeKeyEvent.argtypes = [C.c_void_p, C.c_uint, C.c_int, ulong]
            x.XSetInputFocus(display, windows[0], 2, 0)
            if args.click:
                x.XWarpPointer.argtypes = [C.c_void_p, ulong, ulong, C.c_int, C.c_int,
                                          C.c_uint, C.c_uint, C.c_int, C.c_int]
                xtst.XTestFakeButtonEvent.argtypes = [C.c_void_p, C.c_uint, C.c_int, ulong]
                for point in args.click.split(";"):
                    px, py = [int(v) for v in point.split(",")]
                    if not 0 <= px < 1200 or not 0 <= py < 720:
                        raise ValueError("click must be inside the configured synthetic window")
                    x.XWarpPointer(display, 0, windows[0], 0, 0, 0, 0, px, py)
                    xtst.XTestFakeButtonEvent(display, 1, 1, 0)
                    xtst.XTestFakeButtonEvent(display, 1, 0, 0)
                    x.XFlush(display)
                    time.sleep(1)
            for key in args.keys.split(",") if args.keys else []:
                if key == "ctrl+a":
                    control = x.XKeysymToKeycode(display, x.XStringToKeysym(b"Control_L"))
                    a = x.XKeysymToKeycode(display, x.XStringToKeysym(b"a"))
                    for code, down in [(control, 1), (a, 1), (a, 0), (control, 0)]:
                        xtst.XTestFakeKeyEvent(display, code, down, 0)
                    x.XFlush(display)
                    time.sleep(0.25)
                    continue
                if key not in ["Tab", "space", "Escape", "Return", "Left", "Right", "Up", "Down"]:
                    raise ValueError("unsupported key")
                code = x.XKeysymToKeycode(display, x.XStringToKeysym(key.encode()))
                if not code:
                    raise ValueError("key absent from session keymap")
                xtst.XTestFakeKeyEvent(display, code, 1, 0)
                xtst.XTestFakeKeyEvent(display, code, 0, 0)
                x.XFlush(display)
                time.sleep(0.25)
            if args.text:
                if any(c not in "abcdefghijklmnopqrstuvwxyz0123456789/-_." for c in args.text):
                    raise ValueError("text must be a lowercase ASCII synthetic path")
                for char in args.text:
                    code = x.XKeysymToKeycode(display, ord(char))
                    if not code:
                        raise ValueError("text character absent from session keymap")
                    xtst.XTestFakeKeyEvent(display, code, 1, 0)
                    xtst.XTestFakeKeyEvent(display, code, 0, 0)
                    x.XFlush(display)
                    time.sleep(0.025)
                time.sleep(1)
        x.XGetGeometry.argtypes = [C.c_void_p, ulong, C.POINTER(ulong), C.POINTER(C.c_int),
                                  C.POINTER(C.c_int), C.POINTER(C.c_uint), C.POINTER(C.c_uint),
                                  C.POINTER(C.c_uint), C.POINTER(C.c_uint)]
        x.XGetImage.argtypes = [C.c_void_p, ulong, C.c_int, C.c_int, C.c_uint, C.c_uint, ulong, C.c_int]
        x.XGetImage.restype = C.POINTER(XImage)
        x.XDestroyImage.argtypes = [C.POINTER(XImage)]
        from PIL import Image
        for window in windows:
            root, px, py = ulong(), C.c_int(), C.c_int()
            width, height, border, depth = [C.c_uint() for _ in range(4)]
            if not x.XGetGeometry(display, window, C.byref(root), C.byref(px), C.byref(py),
                                  C.byref(width), C.byref(height), C.byref(border), C.byref(depth)):
                continue
            image = x.XGetImage(display, window, 0, 0, width, height, ulong(-1), 2)
            if not image:
                raise RuntimeError("XGetImage unavailable; no screenshot evidence")
            try:
                value = image.contents
                if value.bits_per_pixel != 32 or value.byte_order != 0:
                    raise RuntimeError("unsupported screenshot pixel layout")
                pixels = C.string_at(value.data, value.bytes_per_line * value.height)
                Image.frombytes("RGB", (value.width, value.height), pixels, "raw", "BGRX",
                                value.bytes_per_line).save(args.output / ("window-%d.png" % window))
            finally:
                x.XDestroyImage(image)
        try:
            import gi
            gi.require_version("Atspi", "2.0")
            from gi.repository import Atspi
            desktop = Atspi.get_desktop(0)
            matching = []
            for i in range(desktop.get_child_count()):
                app = desktop.get_child_at_index(i)
                if app.get_process_id() == args.pid:
                    def inspect(node, depth=0):
                        record = {"name": node.get_name(), "role": node.get_role_name()}
                        if depth < 12:
                            record["children"] = [inspect(node.get_child_at_index(j), depth+1)
                                                  for j in range(node.get_child_count())]
                        return record
                    matching.append(inspect(app))
            (args.output / "accessibility.json").write_text(json.dumps(matching, indent=2) + "\n")
        except (ImportError, ValueError) as error:
            (args.output / "accessibility.json").write_text(json.dumps({"pending": str(error)}))
        print("Captured application-only evidence; evaluate controls, focus and accessibility separately.")
    finally:
        x.XCloseDisplay(display)


if __name__ == "__main__":
    main()
