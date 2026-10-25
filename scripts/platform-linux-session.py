#!/usr/bin/env python3
"""Qualify actual X11/AT-SPI controls in a private, memory-only D-Bus session.

The actual display is retained. The host accessibility bus/settings are never
modified. Run against synthetic fixtures only; native evidence stays external.
"""
import argparse
import importlib.util
import json
import os
import re
from pathlib import Path
import signal
import subprocess
import sys
import time
import uuid

import qualification_stages as stages
from linux_desktop import DesktopMonitor

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("qualification", HERE / "platform-qualification.py")
qualification = importlib.util.module_from_spec(spec)
spec.loader.exec_module(qualification)


def bus_status(env):
    return subprocess.check_output(["gdbus", "call", "--session", "--dest", "org.a11y.Bus",
                                    "--object-path", "/org/a11y/bus", "--method",
                                    "org.freedesktop.DBus.Properties.Get", "org.a11y.Status", "IsEnabled"],
                                   env=env, text=True, timeout=10).strip()


def require_private_bus(env, host):
    if not host or env.get("GSETTINGS_BACKEND") != "memory" or not env.get("DBUS_SESSION_BUS_ADDRESS") or env["DBUS_SESSION_BUS_ADDRESS"] == host:
        raise RuntimeError("Refusing accessibility activation outside private memory-only session")


def worker(binary, root, use_orca=False, observer_mode="attached"):
    def interrupted(_signum, _frame):
        raise RuntimeError("Owned private-session qualification interrupted")
    signal.signal(signal.SIGTERM, interrupted)
    env = dict(os.environ)
    require_private_bus(env, env.get("SCANNER_QUALIFICATION_HOST_BUS"))
    launcher = subprocess.Popen(["/usr/libexec/at-spi-bus-launcher", "--launch-immediately"],
                                env=env, stdout=(root / "atspi.stdout.txt").open("w"),
                                stderr=(root / "atspi.stderr.txt").open("w"), start_new_session=True)
    gui = None
    orca = None
    try:
        deadline = time.monotonic() + 5
        while True:
            owner = subprocess.check_output(["gdbus", "call", "--session", "--dest", "org.freedesktop.DBus",
                                             "--object-path", "/org/freedesktop/DBus", "--method",
                                             "org.freedesktop.DBus.NameHasOwner", "org.a11y.Bus"],
                                            env=env, text=True, timeout=5)
            if "true" in owner:
                break
            if launcher.poll() is not None or time.monotonic() >= deadline:
                raise RuntimeError("Owned private accessibility launcher did not acquire its bus name")
            time.sleep(0.05)
        subprocess.run(["gdbus", "call", "--session", "--dest", "org.a11y.Bus",
                        "--object-path", "/org/a11y/bus", "--method",
                        "org.freedesktop.DBus.Properties.Set", "org.a11y.Status", "IsEnabled", "<true>"],
                       env=env, check=True, capture_output=True, timeout=10)
        gui = subprocess.Popen([str(binary)], env=env, stdin=subprocess.DEVNULL,
                               stdout=(root / "gui.stdout.txt").open("w"),
                               stderr=(root / "gui.stderr.txt").open("w"))
        if use_orca:
            from orca import settings_manager
            preferences = root / "orca-preferences"
            manager = settings_manager.getManager()
            manager.activate(str(preferences), {"enableSpeech": False, "enableBraille": False,
                                                 "enableBrailleMonitor": False})
            manager.setFirstStart(False)
            orca = subprocess.Popen(["orca", "--user-prefs", str(preferences),
                                     "--disable", "speech,braille,braille-monitor", "--debug",
                                     "--debug-file", str(root / "orca-debug.txt")], env=env,
                                    stdout=(root / "orca.stdout.txt").open("w"),
                                    stderr=(root / "orca.stderr.txt").open("w"))
            time.sleep(1)
            if orca.poll() is not None:
                raise RuntimeError("Existing Orca failed to stay running on isolated bus")
        import gi
        gi.require_version("Atspi", "2.0")
        from gi.repository import Atspi, Gio, GLib
        def find(node, name):
            node.clear_cache()
            if node.get_name() == name:
                return node
            for i in range(node.get_child_count()):
                result = find(node.get_child_at_index(i), name)
                if result:
                    return result
            return None
        app = None
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline and app is None:
            desktop = Atspi.get_desktop(0)
            desktop.clear_cache()
            for i in range(desktop.get_child_count()):
                candidate = desktop.get_child_at_index(i)
                if candidate.get_process_id() == gui.pid:
                    app = candidate
                    break
            time.sleep(0.1)
        if app is None:
            raise RuntimeError("Native AT-SPI application unavailable on private enabled bus")
        # Read actual D-Bus method/signal body signatures from this app's
        # native AT-SPI connection, rather than inferring cache compatibility.
        session_bus = Gio.bus_get_sync(Gio.BusType.SESSION, None)
        address = session_bus.call_sync("org.a11y.Bus", "/org/a11y/bus", "org.a11y.Bus", "GetAddress",
                                        None, None, Gio.DBusCallFlags.NONE, 5000, None).unpack()[0]
        native_bus = Gio.DBusConnection.new_for_address_sync(address,
                     Gio.DBusConnectionFlags.AUTHENTICATION_CLIENT | Gio.DBusConnectionFlags.MESSAGE_BUS_CONNECTION, None, None)
        native_bus.set_exit_on_close(False)
        names = native_bus.call_sync("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
                                    "ListNames", None, None, Gio.DBusCallFlags.NONE, 5000, None).unpack()[0]
        destination = None
        for name in names:
            if not name.startswith(":"):
                continue
            pid = native_bus.call_sync("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
                                      "GetConnectionUnixProcessID", GLib.Variant("(s)", (name,)), None,
                                      Gio.DBusCallFlags.NONE, 5000, None).unpack()[0]
            if pid == gui.pid:
                destination = name
                break
        if destination is None:
            raise RuntimeError("Native accessibility bus source PID did not match the owned app")
        cache = native_bus.call_sync(destination, "/org/a11y/atspi/cache", "org.a11y.atspi.Cache", "GetItems",
                                     None, None, Gio.DBusCallFlags.NONE, 5000, None)
        signatures = {"GetItems": cache.get_type_string()[1:-1], "AddAccessible": [], "RemoveAccessible": []}
        def cache_signal(_connection, _sender, _path, _interface, member, parameters, _data):
            if member in ("AddAccessible", "RemoveAccessible"):
                signatures[member].append(parameters.get_type_string()[1:-1])
        subscription = native_bus.signal_subscribe(destination, "org.a11y.atspi.Cache", None,
                         "/org/a11y/atspi/cache", None, Gio.DBusSignalFlags.NONE, cache_signal, None)
        announcements = []
        announcement_subscription = native_bus.signal_subscribe(destination, "org.a11y.atspi.Event.Object", "Announcement",
                         None, None, Gio.DBusSignalFlags.NONE,
                         lambda _c, _s, _p, _i, _m, parameters, _d: announcements.append(parameters.unpack()), None)
        def pump():
            context = GLib.MainContext.default()
            for _ in range(1000):
                if not context.pending():
                    break
                context.iteration(False)
        def press(name):
            deadline = time.monotonic() + 10
            while True:
                app.clear_cache()
                node = find(app, name)
                if node is not None and node.get_state_set().contains(Atspi.StateType.ENABLED) and node.get_state_set().contains(Atspi.StateType.SENSITIVE):
                    break
                if time.monotonic() >= deadline:
                    raise RuntimeError("Native accessible control not present/enabled: " + name)
                time.sleep(0.1)
                pump()
            action = node.get_action_iface()
            if action is None:
                raise RuntimeError("Native AT-SPI actions absent: " + name)
            available = [action.get_action_name(i) for i in range(action.get_n_actions())]
            with (root / "native-actions.jsonl").open("a") as stream:
                stream.write(json.dumps({"name": name, "actions": available}) + "\n")
            index = next((i for i, value in enumerate(available) if value.lower() in ("click", "press", "activate")), None)
            if index is None or not action.do_action(index):
                raise RuntimeError("Native AT-SPI press action failed: " + name)
            time.sleep(0.4)
            pump()
        subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid),
                        "--output", str(root / "initial"), "--keys", "Tab"], env=env, check=True, timeout=20)
        press("Start")
        def observed_status(node, prefix):
            node.clear_cache()
            if node.get_name().startswith(prefix + " | files="):
                return True
            return any(observed_status(node.get_child_at_index(i), prefix) for i in range(node.get_child_count()))
        deadline = time.monotonic() + 10
        while not observed_status(app, "finished"):
            if time.monotonic() >= deadline:
                raise RuntimeError("Native Start did not reach readable finished status")
            time.sleep(0.1)
            pump()
        press("Add Folder")
        deadline = time.monotonic() + 5
        while True:
            entry = find(app, "Folder path: type a path, then Enter or Go")
            component = entry.get_component_iface() if entry is not None else None
            if component is not None and component.grab_focus():
                break
            if time.monotonic() >= deadline:
                subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid),
                                "--output", str(root / "folder-focus-failure")], env=env, check=True, timeout=20)
                subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid),
                                "--output", str(root / "keyboard-folder-diagnostic"), "--keys", "ctrl+o"], env=env, check=True, timeout=20)
                current_cache = native_bus.call_sync(destination, "/org/a11y/atspi/cache", "org.a11y.atspi.Cache", "GetItems", None, None, Gio.DBusCallFlags.NONE, 5000, None).unpack()[0]
                comparison = {"cached_interfaces": list(entry.get_interfaces()) if entry else None, "direct_entries": []}
                for item in current_cache:
                    if item[6] == "Folder path: type a path, then Enter or Go":
                        direct = native_bus.call_sync(destination, item[0][1], "org.a11y.atspi.Accessible", "GetInterfaces", None, None, Gio.DBusCallFlags.NONE, 5000, None).unpack()[0]
                        comparison["direct_entries"].append({"cache_item": item, "direct_interfaces": direct})
                (root / "native-interface-cache-comparison.json").write_text(json.dumps(comparison, indent=2))
                raise RuntimeError("Native editable folder path focus failed; control present=" + str(entry is not None))
            time.sleep(0.1)
            pump()
        current_cache = native_bus.call_sync(destination, "/org/a11y/atspi/cache", "org.a11y.atspi.Cache", "GetItems", None, None, Gio.DBusCallFlags.NONE, 5000, None).unpack()[0]
        (root / "entry-server-cache.json").write_text(json.dumps(current_cache, indent=2))
        invalid = str(root / "nonexistent-folder")
        editable = entry.get_editable_text_iface()
        (root / "editable-interface.json").write_text(json.dumps({"editable_text_available": editable is not None,
                                                                  "interfaces": entry.get_interfaces()}, indent=2))
        if editable is not None:
            if not editable.set_text_contents("aé🙂z"):
                raise RuntimeError("Native Unicode value edit failed")
            text = entry.get_text_iface()
            time.sleep(0.4)
            if not text.set_caret_offset(2):
                raise RuntimeError("Native caret action rejected")
            deadline = time.monotonic() + 3
            while text.get_caret_offset() != 2:
                if time.monotonic() >= deadline:
                    raise RuntimeError("Native Unicode caret did not round trip")
                time.sleep(0.1)
                pump()
            if not text.add_selection(1, 3):
                raise RuntimeError("Native Unicode selection action rejected")
            deadline = time.monotonic() + 3
            while True:
                selected_range = Atspi.Text.get_selection(text, 0)
                if selected_range.start_offset == 1 and selected_range.end_offset == 3:
                    break
                if time.monotonic() >= deadline:
                    raise RuntimeError("Native Unicode selection did not round trip")
                time.sleep(0.1)
                pump()
            if Atspi.Text.get_text(text, 1, 3) != "é🙂":
                raise RuntimeError("Native selection used byte or UTF-16 indexes")
            (root / "text-selection.json").write_text(json.dumps({"caret": 2, "selection": [1, 3],
                    "selected": "é🙂", "unicode_scalar_offsets": True, "passed": True}, ensure_ascii=False, indent=2))
            if not editable.set_text_contents(invalid):
                raise RuntimeError("Native folder path editing failed")
        else:
            subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid),
                            "--output", str(root / "native-focus-keyboard-edit"), "--keys", "ctrl+a",
                            "--text", invalid], env=env, check=True, timeout=20)
        press("Go")
        subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid),
                        "--output", str(root / "folder-error")], env=env, check=True, timeout=20)
        error_snapshot = json.dumps(json.loads((root / "folder-error/accessibility.json").read_text()))
        native_error_status = "Cannot open folder" in error_snapshot and invalid in error_snapshot
        deadline = time.monotonic() + 3
        while not any("Cannot open folder" in str(event) and invalid in str(event) for event in announcements):
            if time.monotonic() >= deadline:
                raise RuntimeError("Native affected-folder error announcement missing")
            time.sleep(0.1)
            pump()
        (root / "native-announcements.json").write_text(json.dumps(announcements, ensure_ascii=False, indent=2))
        native_text = entry.get_text_iface() is not None
        selected = root / "scan-root" / "selected-root"
        selected.mkdir()
        if editable is not None:
            if not editable.set_text_contents(str(selected)):
                raise RuntimeError("Native valid folder editing failed")
        else:
            if not entry.get_component_iface().grab_focus():
                raise RuntimeError("Native folder edit refocus failed")
            subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid),
                            "--output", str(root / "native-valid-keyboard-edit"), "--keys", "ctrl+a",
                            "--text", str(selected)], env=env, check=True, timeout=20)
        press("Go")
        time.sleep(1)
        pump()
        subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid), "--output", str(root / "preselection")], env=env, check=True, timeout=20)
        fresh_cache = native_bus.call_sync(destination, "/org/a11y/atspi/cache", "org.a11y.atspi.Cache", "GetItems", None, None, Gio.DBusCallFlags.NONE, 5000, None).unpack()[0]
        (root / "preselection/direct-cache.json").write_text(json.dumps(fresh_cache, indent=2))
        press("Select Folder")
        deadline = time.monotonic() + 10
        while find(app, "Folder path: type a path, then Enter or Go") is not None:
            if time.monotonic() >= deadline:
                subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid), "--output", str(root / "selection-failure")], env=env, check=True, timeout=20)
                fresh_cache = native_bus.call_sync(destination, "/org/a11y/atspi/cache", "org.a11y.atspi.Cache", "GetItems", None, None, Gio.DBusCallFlags.NONE, 5000, None).unpack()[0]
                (root / "selection-failure/direct-cache.json").write_text(json.dumps(fresh_cache, indent=2))
                raise RuntimeError("Native folder selection did not close the dialog")
            time.sleep(0.1)
            pump()
        subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid),
                        "--output", str(root / "tree"), "--keys", "ctrl+l,Down,space"], env=env, check=True, timeout=20)
        def flatten(nodes):
            for node in nodes:
                yield node
                yield from flatten(node.get("children", []))
        tree_nodes = list(flatten(json.loads((root / "tree/accessibility.json").read_text())))
        selected_rows = [n for n in tree_nodes if n["role"] == "tree item" and n.get("selected")]
        focused_rows = [n for n in tree_nodes if n["role"] in ("tree", "tree item") and n.get("focused")]
        if len(selected_rows) != 1 or not focused_rows:
            raise RuntimeError("Native keyboard tree selection/focus not exposed")
        selected_path = re.sub(r" \((enabled|disabled)\)$", "", selected_rows[0]["name"])
        if not any(selected_path in (n.get("text", "") or n.get("name", "")) and
                   ("Files:" in (n.get("text", "") or n.get("name", "")) or "(no data yet)" in (n.get("text", "") or n.get("name", ""))) for n in tree_nodes):
            raise RuntimeError("Native keyboard selection did not update readable folder details")
        profile_component = find(app, "Profile: Qualification").get_component_iface()
        if profile_component is None or not profile_component.grab_focus():
            raise RuntimeError("Native profile button focus failed")
        time.sleep(0.3)
        subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid), "--output", str(root / "profile-focused")], env=env, check=True, timeout=20)
        subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid),
                        "--output", str(root / "profile-open"), "--keys", "ctrl+b"], env=env, check=True, timeout=20)
        press("Use profile Secondary")
        settings = qualification.settings_directory(root) / "settings.yaml"
        if "active_profile: Secondary" not in settings.read_text():
            raise RuntimeError("Native profile choice did not persist")
        press("Profile: Secondary")
        press("Use profile Qualification")
        if "active_profile: Qualification" not in settings.read_text():
            raise RuntimeError("Native original profile was not restored")
        settings = qualification.settings_directory(root) / "settings.yaml"
        if str(selected) not in settings.read_text():
            raise RuntimeError("Native selected root was not persisted")
        settings_before = settings.read_bytes()
        press("Settings")
        subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid),
                        "--output", str(root / "settings-window"), "--title", "Settings", "--keys", "Tab",
                        "--close-settings"], env=env, check=True, timeout=20)
        settings_snapshot = (root / "settings-window/accessibility.json").read_text()
        if "Browse root 1" not in settings_snapshot or str(root / "scan-root") not in settings_snapshot:
            raise RuntimeError("Native Settings controls/root were not captured")
        deadline = time.monotonic() + 5
        while find(app, "Browse root 1") is not None:
            if gui.poll() is not None or time.monotonic() >= deadline:
                raise RuntimeError("Owned Settings close did not preserve the running main window")
            time.sleep(.05)
            pump()
        if settings.read_bytes() != settings_before:
            raise RuntimeError("Opening/closing Settings changed the saved profile")
        subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid),
                        "--output", str(root / "after-settings"), "--keys", "ctrl+l"], env=env, check=True, timeout=20)
        # A tiny owned FIFO holds active extraction without a large workload.
        fifo = root / "scan-root" / "0-controlled-read"
        os.mkfifo(fifo)
        holder = os.open(fifo, os.O_RDWR | os.O_NONBLOCK)
        def wait_status(prefix):
            deadline = time.monotonic() + 10
            while not observed_status(app, prefix):
                if time.monotonic() >= deadline:
                    raise RuntimeError("Native lifecycle status absent: " + prefix)
                time.sleep(0.1)
                pump()
        try:
            press("Start")
            press("Pause")
            wait_status("paused")
            press("Resume")
            wait_status("running")
            press("Cancel")
            wait_status("canceling")
        finally:
            os.close(holder)
            fifo.unlink()
        wait_status("canceled")
        subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid), "--output", str(root / "canceled")], env=env, check=True, timeout=20)
        # Restart only the owned process; preserve exactly this fixture's settings.
        settings_before = settings.read_bytes()
        gui.terminate()
        gui.wait(timeout=5)
        gui = subprocess.Popen([str(binary)], env=env, stdin=subprocess.DEVNULL,
                               stdout=(root / "restart.stdout.txt").open("w"), stderr=(root / "restart.stderr.txt").open("w"))
        app = None
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline and app is None:
            desktop = Atspi.get_desktop(0)
            desktop.clear_cache()
            for i in range(desktop.get_child_count()):
                candidate = desktop.get_child_at_index(i)
                if candidate.get_process_id() == gui.pid:
                    app = candidate
                    break
            time.sleep(0.1)
        if app is None or settings.read_bytes() != settings_before:
            raise RuntimeError("Native process restart/settings preservation failed")
        press("Start (Resume)")
        wait_status("finished")
        subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid), "--output", str(root / "restarted")], env=env, check=True, timeout=20)
        restart_snapshot = (root / "restarted/accessibility.json").read_text()
        if "Resumed partial inventory (inclusive checkpoint cursor)" not in restart_snapshot:
            raise RuntimeError("Native restarted resume did not disclose partial inventory")
        report = {"schema": 1, "private_memory_bus": True, "native_process_matched": True,
                  "native_atspi_press": True, "native_focus": True, "native_editable_text_interface": editable is not None,
                  "keyboard_edit": True, "keyboard_tree_focus_selection_details": True,
                  "native_profile_switch_restore": True, "native_pause_resume_cancel": True,
                  "native_settings_window_keyboard_close": True, "settings_unchanged_after_close": True,
                  "native_process_restart_settings_partial_resume": True,
                  "renderer": "software" if env.get("LIBGL_ALWAYS_SOFTWARE") == "1" else "host-default",
                  "selected_root_persisted": True, "native_text_interface": native_text,
                  "native_error_status": native_error_status,
                  "passed": native_text and native_error_status and editable is not None, "release_acceptance": False}
        pump()
        signatures["AddAccessible"] = sorted(set(signatures["AddAccessible"]))
        signatures["RemoveAccessible"] = sorted(set(signatures["RemoveAccessible"]))
        cache_ok = (signatures["GetItems"] == "a((so)(so)(so)iiassusau)" and
                    signatures["AddAccessible"] == ["((so)(so)(so)iiassusau)"] and
                    signatures["RemoveAccessible"] == ["(so)"])
        (root / "native-cache-wire.json").write_text(json.dumps(signatures, indent=2) + "\n")
        report["native_cache_wire"] = cache_ok
        report["passed"] = report["passed"] and cache_ok
        if use_orca:
            time.sleep(1)
            log = (root / "orca-debug.txt").read_text(errors="replace")
            report["orca"] = {"running": orca.poll() is None, "speech": False, "braille": False,
                              "app_observed_in_log": "Scanner" in log or binary.name in log,
                              "error_observed_in_log": "Cannot open folder" in log,
                              "tree_focus_observed_in_log": "Locus of focus is [tree item:" in log,
                              "cache_signature_warnings": log.count("unknown signature") + log.count("Unknown signature"),
                              "parent_lookup_exceptions": log.count("Exception in get_parent"),
                              "announcements_and_caret": "not qualified"}
            report["passed"] = report["passed"] and all(report["orca"][k] for k in ["running", "app_observed_in_log", "error_observed_in_log", "tree_focus_observed_in_log"]) and report["orca"]["cache_signature_warnings"] == 0 and report["orca"]["parent_lookup_exceptions"] == 0
        if observer_mode == "detach-before-quit":
            stage = stages.begin(root, env["SCANNER_QUALIFICATION_NONCE"], "observer-detach")
            native_bus.signal_unsubscribe(subscription)
            native_bus.signal_unsubscribe(announcement_subscription)
            pump()
            report["observer_detach_ms"] = stages.finish(root, stage)
        stage = stages.begin(root, env["SCANNER_QUALIFICATION_NONCE"], "native-quit")
        press("Quit")
        if gui.wait(timeout=8) != 0:
            raise RuntimeError("Owned native Quit returned a nonzero exit")
        quit_ms = stages.finish(root, stage)
        report["native_quit"] = {"action": "AT-SPI Quit", "exit_code": gui.returncode,
                                 "forced": False, "total_ms": quit_ms, "limit_ms": 8000}
        report["observer_mode"] = observer_mode
        if observer_mode == "attached":
            native_bus.signal_unsubscribe(subscription)
            native_bus.signal_unsubscribe(announcement_subscription)
        for log_name in ("gui.stderr.txt", "restart.stderr.txt"):
            if "Error in Fyne call thread" in (root / log_name).read_text(errors="replace"):
                raise RuntimeError("Fyne wrong-thread diagnostic observed")
        (root / "worker-report.json").write_text(json.dumps(report, indent=2) + "\n")
        return report
    finally:
        if "signatures" in locals():
            (root / "native-cache-wire.json").write_text(json.dumps(signatures, indent=2) + "\n")
        if orca is not None and orca.poll() is None:
            orca.terminate()
            orca.wait(timeout=10)
        if gui is not None and gui.poll() is None:
            gui.terminate()
            try:
                gui.wait(timeout=3)
            except subprocess.TimeoutExpired:
                # Owned diagnostic process only: preserve Go goroutine evidence
                # before killing a GUI that ignored termination.
                gui.send_signal(signal.SIGQUIT)
                try:
                    gui.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    gui.kill()
                    gui.wait(timeout=5)
                (root / "gui-termination-pending.json").write_text(json.dumps({"normal_termination": False,
                        "diagnostic_quit_sent": True, "exit_code": gui.returncode}, indent=2))
        if launcher.poll() is None:
            os.killpg(launcher.pid, signal.SIGTERM)
            launcher.wait(timeout=10)


def renderer_environment(parent, mode):
    env = dict(parent)
    if mode == "hardware-no-vsync":
        env.pop("LIBGL_ALWAYS_SOFTWARE", None)
        env["vblank_mode"] = "0"
    elif mode == "software":
        env["LIBGL_ALWAYS_SOFTWARE"] = "1"
    elif mode != "default":
        raise ValueError("Unknown rendering mode")
    return env


def run(binary, root, use_orca=False, rendering="default", observer_mode="attached"):
    env = qualification.prepare(root)
    env = renderer_environment(env, rendering)
    if rendering == "hardware-no-vsync":
        renderer = subprocess.check_output(["glxinfo", "-B"], env=env, text=True, timeout=10)
        (root / "renderer.txt").write_text(renderer)
        if "Accelerated: yes" not in renderer or re.search(r"OpenGL renderer string:.*(?:llvmpipe|softpipe|Software)", renderer, re.I):
            raise RuntimeError("Hardware qualification requires an accelerated renderer")
    env["GSETTINGS_BACKEND"] = "memory"
    runtime = root / "runtime"
    runtime.mkdir(mode=0o700)
    env["XDG_RUNTIME_DIR"] = str(runtime)
    host = os.environ.get("DBUS_SESSION_BUS_ADDRESS", "unix:path=/run/user/%d/bus" % os.getuid())
    host_env = dict(os.environ, DBUS_SESSION_BUS_ADDRESS=host)
    before = bus_status(host_env)
    env["SCANNER_QUALIFICATION_HOST_BUS"] = host
    nonce = uuid.uuid4().hex
    env["SCANNER_QUALIFICATION_NONCE"] = nonce
    desktop = DesktopMonitor(host_env, root)
    try:
        desktop.start()
        with (root / "session.stdout.txt").open("w") as stdout, (root / "session.stderr.txt").open("w") as stderr:
            session = subprocess.Popen(["dbus-run-session", "--", sys.executable, str(Path(__file__).resolve()),
                                        "--binary", str(binary), "--fixture", str(root), "--private-session",
                                        "--observer-mode", observer_mode] + (["--orca"] if use_orca else []),
                                       env=env, stdout=stdout, stderr=stderr, start_new_session=True)
            deadline = time.monotonic() + 90
            try:
                while True:
                    desktop.check()
                    if stages.expired(root, nonce):
                        raise RuntimeError("Owned native stage exceeded eight-second total deadline")
                    if session.poll() is not None:
                        break
                    if time.monotonic() >= deadline:
                        raise RuntimeError("Owned private session timed out")
                    time.sleep(.05)
            finally:
                if session.poll() is None:
                    os.killpg(session.pid, signal.SIGTERM)
                    try:
                        session.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        os.killpg(session.pid, signal.SIGKILL)
                        session.wait(timeout=5)
    finally:
        desktop.close()
        after = bus_status(host_env)
        (root / "host-status.json").write_text(json.dumps({"before": before, "after": after,
                                                          "unchanged": before == after}, indent=2) + "\n")
    if before != after:
        raise RuntimeError("Host accessibility state changed unexpectedly")
    desktop.check()
    if session.returncode:
        raise RuntimeError("Native private-session qualification failed; inspect private logs")
    report = json.loads((root / "worker-report.json").read_text())
    report["desktop_availability"] = "GNOME unlocked and displays powered on throughout observed run"
    report["rendering"] = {"mode": rendering, "child_vblank_mode": env.get("vblank_mode"),
                           "child_software_override": env.get("LIBGL_ALWAYS_SOFTWARE"),
                           "host_environment_changed": False}
    (root / "session-report.json").write_text(json.dumps(report, indent=2) + "\n")
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--fixture", type=Path, required=True)
    parser.add_argument("--orca", action="store_true", help="also exercise existing Orca silently in this isolated session")
    parser.add_argument("--rendering", choices=("default", "software", "hardware-no-vsync"), default="default",
                        help="explicit child-only renderer mode; hardware-no-vsync requires accelerated glxinfo")
    parser.add_argument("--observer-mode", choices=("attached", "detach-before-quit"), default="attached")
    parser.add_argument("--private-session", action="store_true", help=argparse.SUPPRESS)
    args = parser.parse_args()
    args.binary, args.fixture = args.binary.resolve(), args.fixture.resolve()
    if args.private_session:
        report = worker(args.binary, args.fixture, args.orca, args.observer_mode)
        raise SystemExit(0 if report["passed"] else 1)
    else:
        print(json.dumps(run(args.binary, args.fixture, args.orca, args.rendering, args.observer_mode), indent=2))
