#!/usr/bin/env python3
import argparse
import hashlib
import importlib.util
import json
import os
import re
import subprocess
import time
from pathlib import Path

parser = argparse.ArgumentParser(
    description="Replay native AX, keyboard and lifecycle on an owned synthetic Mac process."
)
parser.add_argument("--fixture", type=Path, required=True)
parser.add_argument("--binary", type=Path, required=True)
parser.add_argument("--helper", type=Path, required=True)
parser.add_argument("--revision", required=True)
args = parser.parse_args()
if not re.fullmatch(r"[0-9a-f]{40}", args.revision):
    parser.error("revision must be the full reviewed source SHA")
s = Path(__file__).resolve().parents[1]
sp = importlib.util.spec_from_file_location(
    "q", s / "scripts/platform-qualification.py"
)
q = importlib.util.module_from_spec(sp)
sp.loader.exec_module(q)
f = args.fixture.resolve()
env = q.prepare(f)
binary = args.binary.resolve()
helper = args.helper.resolve()
holder = None
fifo = None
p = subprocess.Popen(
    [str(binary)],
    env=env,
    stdout=(f / "stdout.txt").open("w"),
    stderr=(f / "stderr.txt").open("w"),
)


def call(name, actions="", title=None, expected=None):
    cmd = [str(helper), str(p.pid), str(f / name), actions]
    if title or expected:
        cmd.append(title or "Scanner")
    if expected:
        cmd.append("--expect:" + expected)
    subprocess.run(
        cmd,
        check=True,
        timeout=20,
        stdout=(f / (name + ".stdout.txt")).open("w"),
        stderr=(f / (name + ".stderr.txt")).open("w"),
    )
    return json.loads((f / name / "accessibility-after.json").read_text())


def nodes(v):
    yield v
    for c in v.get("children", []):
        yield from nodes(c)


def require(v, fragment):
    assert any(
        fragment in (n.get("AXTitle", "") + " " + n.get("AXValue", ""))
        for n in nodes(v)
    ), fragment


try:
    time.sleep(3)
    title = "Folder path: type a path, then Enter or Go"
    v = call("initial", "tab")
    require(v, "Start")
    require(v, "Settings")
    v = call("start", "cmd+s", expected="finished | files=")
    require(v, "finished | files=")
    v = call("folder", "cmd+o")
    require(v, title)
    v = call(
        "unicode",
        f"axfocus:{title};axvalue|{title}|aé🙂z;axselect|{title}|1|3;axselect|{title}|2|0",
    )
    assert (
        json.loads((f / "unicode/native-text-selection-1-3.json").read_text())[
            "selected_text"
        ]
        == "é🙂"
    )
    invalid = f / "nonexistent-folder"
    v = call("error", f"axvalue|{title}|{invalid};axpress:Go")
    require(v, "Cannot open folder")
    require(v, str(invalid))
    selected = f / "scan-root/selected-root"
    selected.mkdir()
    v = call("select", f"axvalue|{title}|{selected};axpress:Go;axpress:Select Folder")
    assert not any(n.get("AXTitle") == title for n in nodes(v))
    settings = q.settings_directory(f) / "settings.yaml"
    assert str(selected) in settings.read_text()
    v = call("tree", "cmd+l,down,space")
    rows = [
        n for n in nodes(v) if n.get("AXRole") == "AXRow" and n.get("AXSelected") == "1"
    ]
    assert len(rows) == 1, rows
    assert any(n.get("AXFocused") == "1" for n in nodes(v))
    selected_path = rows[0].get("AXTitle", "").split(" (")[0]
    assert any(
        selected_path in n.get("AXValue", "")
        and (
            "Files:" in n.get("AXValue", "") or "(no data yet)" in n.get("AXValue", "")
        )
        for n in nodes(v)
    ), "selected row must match readable details"
    v = call("profile", "cmd+b;axpress:Use profile Secondary")
    assert "active_profile: Secondary" in settings.read_text()
    v = call("restore", "cmd+b;axpress:Use profile Qualification")
    assert "active_profile: Qualification" in settings.read_text()
    fifo = f / "scan-root/0-controlled-read"
    os.mkfifo(fifo)
    holder = os.open(fifo, os.O_RDWR | os.O_NONBLOCK)
    v = call("pause", "cmd+s,cmd+p", expected="paused | files=")
    require(v, "paused | files=")
    v = call("resume", "cmd+r", expected="running | files=")
    require(v, "running | files=")
    v = call("cancel", "cmd+k", expected="canceling | files=")
    require(v, "canceling | files=")
    os.close(holder)
    holder = None
    fifo.unlink()
    fifo = None
    v = call("canceled", expected="canceled | files=")
    require(v, "canceled | files=")
    saved = settings.read_bytes()
    # Native exit, not forced termination, before restarting settings/checkpoint.
    start = time.monotonic()
    subprocess.run(
        [str(helper), str(p.pid), str(f / "quit"), "cmd+q"], check=True, timeout=8
    )
    assert p.wait(timeout=max(0.01, 8 - (time.monotonic() - start))) == 0
    quit_ms = (time.monotonic() - start) * 1000
    p = subprocess.Popen(
        [str(binary)],
        env=env,
        stdout=(f / "restart.stdout.txt").open("w"),
        stderr=(f / "restart.stderr.txt").open("w"),
    )
    time.sleep(3)
    assert settings.read_bytes() == saved
    v = call("restarted", "cmd+shift+s", expected="finished | files=")
    require(v, "finished | files=")
    require(v, "Resumed partial inventory (inclusive checkpoint cursor)")
    start = time.monotonic()
    subprocess.run(
        [str(helper), str(p.pid), str(f / "restart-quit"), "axpress:Quit"],
        check=True,
        timeout=8,
    )
    assert p.wait(timeout=max(0.01, 8 - (time.monotonic() - start))) == 0
    for n in ("stderr.txt", "restart.stderr.txt"):
        assert "Error in Fyne call thread" not in (f / n).read_text()
    (f / "report.json").write_text(
        json.dumps(
            {
                "passed": True,
                "source": args.revision,
                "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
                "unicode": True,
                "folder_error_recovery": True,
                "profile_switch_restore": True,
                "tree_keyboard": True,
                "pause_resume_cancel": True,
                "settings_restart_checkpoint": True,
                "native_quit_exit": 0,
                "quit_ms": quit_ms,
                "forced_cleanup": False,
                "release_acceptance": False,
            },
            indent=2,
        )
        + "\n"
    )
finally:
    if holder is not None:
        os.close(holder)
    if fifo:
        fifo.unlink(missing_ok=True)
    if p.poll() is None:
        p.kill()
        p.wait()
        (f / "forced-cleanup.json").write_text(
            json.dumps({"pid": p.pid, "forced": True})
        )
