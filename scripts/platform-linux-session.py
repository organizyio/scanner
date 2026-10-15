#!/usr/bin/env python3
"""Qualify actual X11/AT-SPI controls in a private, memory-only D-Bus session.

The actual display is retained. The host accessibility bus/settings are never
modified. Run against synthetic fixtures only; native evidence stays external.
"""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time

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
    if env.get("GSETTINGS_BACKEND") != "memory" or not env.get("DBUS_SESSION_BUS_ADDRESS") or env["DBUS_SESSION_BUS_ADDRESS"] == host:
        raise RuntimeError("Refusing accessibility activation outside private memory-only session")


def worker(binary, root):
    def interrupted(_signum, _frame):
        raise RuntimeError("Owned private-session qualification interrupted")
    signal.signal(signal.SIGTERM, interrupted)
    env = dict(os.environ)
    require_private_bus(env, env.get("SCANNER_QUALIFICATION_HOST_BUS"))
    launcher = subprocess.Popen(["/usr/libexec/at-spi-bus-launcher", "--launch-immediately"],
                                env=env, stdout=(root / "atspi.stdout.txt").open("w"),
                                stderr=(root / "atspi.stderr.txt").open("w"), start_new_session=True)
    gui = None
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
        import gi
        gi.require_version("Atspi", "2.0")
        from gi.repository import Atspi
        def find(node, name):
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
        def press(name):
            app.clear_cache()
            node = find(app, name)
            if node is None:
                raise RuntimeError("Missing native accessible control: " + name)
            action = node.get_action_iface()
            if action is None or not action.do_action(0):
                raise RuntimeError("Native AT-SPI action failed: " + name)
            time.sleep(0.4)
        subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid),
                        "--output", str(root / "initial"), "--keys", "Tab"], env=env, check=True, timeout=20)
        press("Start")
        press("Add Folder")
        entry = find(app, "Folder path: type a path, then Enter or Go")
        if entry is None or not entry.get_component_iface().grab_focus():
            raise RuntimeError("Native editable folder path focus failed")
        invalid = str(root / "nonexistent-folder")
        editable = entry.get_editable_text_iface()
        (root / "editable-interface.json").write_text(json.dumps({"editable_text_available": editable is not None,
                                                                  "interfaces": entry.get_interfaces()}, indent=2))
        if editable is not None:
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
        press("Select Folder")
        subprocess.run([sys.executable, str(HERE / "platform-x11.py"), "--pid", str(gui.pid),
                        "--output", str(root / "tree"), "--keys", "ctrl+l,Down,space"], env=env, check=True, timeout=20)
        settings = qualification.settings_directory(root) / "settings.yaml"
        if str(selected) not in settings.read_text():
            raise RuntimeError("Native selected root was not persisted")
        report = {"schema": 1, "private_memory_bus": True, "native_process_matched": True,
                  "native_atspi_press": True, "native_focus": True, "native_editable_text_interface": editable is not None,
                  "keyboard_edit": True,
                  "selected_root_persisted": True, "native_text_interface": native_text,
                  "native_error_status": native_error_status,
                  "passed": native_text and native_error_status and editable is not None, "release_acceptance": False}
        (root / "session-report.json").write_text(json.dumps(report, indent=2) + "\n")
        return report
    finally:
        if gui is not None and gui.poll() is None:
            gui.terminate()
            gui.wait(timeout=10)
        if launcher.poll() is None:
            os.killpg(launcher.pid, signal.SIGTERM)
            launcher.wait(timeout=10)


def run(binary, root):
    env = qualification.prepare(root)
    env["GSETTINGS_BACKEND"] = "memory"
    runtime = root / "runtime"
    runtime.mkdir(mode=0o700)
    env["XDG_RUNTIME_DIR"] = str(runtime)
    host = os.environ.get("DBUS_SESSION_BUS_ADDRESS", "unix:path=/run/user/%d/bus" % os.getuid())
    host_env = dict(os.environ, DBUS_SESSION_BUS_ADDRESS=host)
    before = bus_status(host_env)
    env["SCANNER_QUALIFICATION_HOST_BUS"] = host
    try:
        session = subprocess.Popen(["dbus-run-session", "--", sys.executable, str(Path(__file__).resolve()),
                                    "--binary", str(binary), "--fixture", str(root), "--private-session"],
                                   env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                   text=True, start_new_session=True)
        try:
            stdout, stderr = session.communicate(timeout=90)
        except subprocess.TimeoutExpired:
            os.killpg(session.pid, signal.SIGTERM)
            try:
                stdout, stderr = session.communicate(timeout=10)
            except subprocess.TimeoutExpired:
                os.killpg(session.pid, signal.SIGKILL)
                stdout, stderr = session.communicate(timeout=5)
            raise RuntimeError("Owned private session timed out and was stopped")
        finally:
            (root / "session.stdout.txt").write_text(stdout if "stdout" in locals() else "")
            (root / "session.stderr.txt").write_text(stderr if "stderr" in locals() else "")
    finally:
        after = bus_status(host_env)
        (root / "host-status.json").write_text(json.dumps({"before": before, "after": after,
                                                          "unchanged": before == after}, indent=2) + "\n")
    if before != after:
        raise RuntimeError("Host accessibility state changed unexpectedly")
    if session.returncode:
        raise RuntimeError("Native private-session qualification failed; inspect private logs")
    return json.loads((root / "session-report.json").read_text())


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--fixture", type=Path, required=True)
    parser.add_argument("--private-session", action="store_true", help=argparse.SUPPRESS)
    args = parser.parse_args()
    args.binary, args.fixture = args.binary.resolve(), args.fixture.resolve()
    if args.private_session:
        report = worker(args.binary, args.fixture)
        raise SystemExit(0 if report["passed"] else 1)
    else:
        print(json.dumps(run(args.binary, args.fixture), indent=2))
