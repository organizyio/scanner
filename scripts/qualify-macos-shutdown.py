#!/usr/bin/env python3
"""Owned native AX observer/Quit matrix; no speech, permissions or host changes."""

import argparse
import errno
import hashlib
import importlib.util
import json
import os
import re
import signal
import subprocess
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location(
    "qualification", HERE / "platform-qualification.py"
)
qualification = importlib.util.module_from_spec(spec)
spec.loader.exec_module(qualification)


def run_case(args, name, mode, scenario):
    root = args.evidence / name
    env = qualification.prepare(root)
    fifo = holder = observer = None
    if scenario == "active":
        fifo = root / "scan-root/0-controlled-read"
        os.mkfifo(fifo)
    gui = subprocess.Popen(
        [str(args.binary)],
        env=env,
        stdout=(root / "stdout.txt").open("w"),
        stderr=(root / "stderr.txt").open("w"),
    )
    observer_root = root / "observer"

    def observe():
        nonlocal observer
        observer = subprocess.Popen(
            [str(args.observer), str(gui.pid), str(observer_root)],
            stdout=(root / "observer.stdout.txt").open("w"),
            stderr=(root / "observer.stderr.txt").open("w"),
        )
        deadline = time.monotonic() + 5
        while not (observer_root / "ready.json").exists():
            if observer.poll() is not None or time.monotonic() >= deadline:
                raise RuntimeError("owned AX observer failed to register")
            time.sleep(0.05)
        if json.loads((observer_root / "ready.json").read_text())["pid"] != gui.pid:
            raise RuntimeError("AX observer PID mismatch")

    def action(label, keys, title="Scanner", timeout=20):
        subprocess.run(
            [str(args.helper), str(gui.pid), str(root / label), keys, title],
            check=True,
            timeout=timeout,
            stdout=(root / (label + ".stdout.txt")).open("w"),
            stderr=(root / (label + ".stderr.txt")).open("w"),
        )

    try:
        time.sleep(3)
        if mode != "none":
            observe()
        action("focus", "cmd+l,down,space")
        if scenario == "settings":
            action("settings", "cmd+comma")
        elif scenario == "modal":
            action("modal", "cmd+o")
        elif scenario == "active":
            action("active", "cmd+s")
            snapshot = (root / "active/accessibility-after.json").read_text()
            if "running | files=" not in snapshot:
                raise RuntimeError("active fixture did not enter running state")
            read_deadline = time.monotonic() + 5
            while holder is None:
                try:
                    # A writer can open only once Scanner's reader is present;
                    # an owned RDWR handle would incorrectly satisfy this proof.
                    holder = os.open(fifo, os.O_WRONLY | os.O_NONBLOCK)
                except OSError as error:
                    if error.errno != errno.ENXIO or time.monotonic() >= read_deadline:
                        raise RuntimeError(
                            "Scanner did not enter the owned FIFO read"
                        ) from error
                    time.sleep(0.02)
        detach_ms = None
        if mode == "detach-before-quit":
            start = time.monotonic()
            (observer_root / "detach").touch()
            if observer.wait(timeout=8) != 0:
                raise RuntimeError("AX observer detach failed")
            detach_ms = (time.monotonic() - start) * 1000
            if detach_ms >= 8000:
                raise RuntimeError("AX observer detach exceeded deadline")
        keys = "cmd+q" if scenario == "modal" or mode == "none" else "axpress:Quit"
        title = "Settings" if scenario == "settings" else "Scanner"
        start = time.monotonic()
        helper = subprocess.Popen(
            [str(args.helper), str(gui.pid), str(root / "quit"), keys, title],
            stdout=(root / "quit.stdout.txt").open("w"),
            stderr=(root / "quit.stderr.txt").open("w"),
        )
        try:
            if scenario == "active":
                # Prove Quit was actually invoked before releasing the read;
                # helper startup/lookup time cannot stand in for I/O waiting.
                invocation_deadline = time.monotonic() + 5
                trace = root / "quit/native-stages.json"
                while not trace.exists() or not any(
                    value["stage"] == "native-press-returned"
                    for value in json.loads(trace.read_text())
                ):
                    if (
                        helper.poll() is not None
                        or time.monotonic() >= invocation_deadline
                    ):
                        raise RuntimeError(
                            "Quit was not invoked before active-read release"
                        )
                    time.sleep(0.02)
                time.sleep(0.25)
                if gui.poll() is not None:
                    raise RuntimeError("Quit preempted active extraction")
                os.write(holder, b"synthetic active-read qualification\n")
                os.close(holder)
                holder = None
            if helper.wait(timeout=max(0.01, 8 - (time.monotonic() - start))) != 0:
                raise RuntimeError("native Quit invocation failed")
            if gui.wait(timeout=max(0.01, 8 - (time.monotonic() - start))) != 0:
                raise RuntimeError("native Quit returned a nonzero exit")
            quit_ms = (time.monotonic() - start) * 1000
            if quit_ms >= 8000:
                raise RuntimeError("total native Quit exceeded deadline")
        finally:
            if helper.poll() is None:
                helper.kill()
                helper.wait(timeout=5)
        if observer is not None:
            if observer.wait(timeout=8) != 0:
                raise RuntimeError("owned AX observer did not finish")
            observed = json.loads((observer_root / "detached.json").read_text())
            if not observed["events"]:
                raise RuntimeError(
                    "registered AX observer never received a native event"
                )
        if scenario == "active":
            checkpoint = json.loads((root / "checkpoint.json").read_text())
            if checkpoint.get("last_path") != str(fifo) or not checkpoint.get(
                "updated_at"
            ):
                raise RuntimeError(
                    "active Quit did not flush the completed read's checkpoint"
                )
        if "Error in Fyne call thread" in (root / "stderr.txt").read_text():
            raise RuntimeError("Fyne wrong-thread warning")
        report = {
            "passed": True,
            "observer_mode": mode,
            "scenario": scenario,
            "quit_method": keys,
            "quit_ms": quit_ms,
            "observer_detach_ms": detach_ms,
            "exit_code": gui.returncode,
            "forced": False,
            "active_extraction_waited": scenario == "active",
        }
        (root / "report.json").write_text(json.dumps(report, indent=2) + "\n")
        return report
    except Exception as error:
        (root / "failure.json").write_text(
            json.dumps({"passed": False, "error": str(error)}) + "\n"
        )
        raise
    finally:
        if holder is not None:
            os.close(holder)
        if fifo is not None:
            fifo.unlink(missing_ok=True)
        if gui.poll() is None:
            # Only after failure: preserve the owned process's Go stacks before
            # cleanup. Diagnostic exit or forced cleanup never counts as a pass.
            gui.send_signal(signal.SIGQUIT)
            try:
                gui.wait(timeout=3)
            except subprocess.TimeoutExpired:
                gui.kill()
                gui.wait(timeout=5)
            (root / "forced-cleanup.json").write_text(
                json.dumps({"pid": gui.pid, "forced": True})
            )
        if observer is not None and observer.poll() is None:
            (observer_root / "detach").touch()
            try:
                observer.wait(timeout=8)
            except subprocess.TimeoutExpired:
                observer.kill()
                observer.wait(timeout=5)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for key in ("binary", "helper", "observer", "evidence"):
        parser.add_argument("--" + key, type=Path, required=True)
    parser.add_argument("--revision", required=True)
    parser.add_argument("--binary-sha256", required=True)
    args = parser.parse_args()
    if not re.fullmatch(r"[0-9a-f]{40}", args.revision):
        parser.error("revision must be a full reviewed source SHA")
    for key in ("binary", "helper", "observer", "evidence"):
        setattr(args, key, getattr(args, key).resolve())
    if hashlib.sha256(args.binary.read_bytes()).hexdigest() != args.binary_sha256:
        parser.error("binary digest does not match reviewed artifact")
    if args.evidence.is_relative_to(HERE.parent):
        parser.error("private evidence must remain outside the checkout")
    args.evidence.mkdir(parents=True, exist_ok=False)
    results = []
    for mode in ("attached", "detach-before-quit", "none"):
        for number in range(3):
            results.append(run_case(args, f"{mode}-{number + 1}", mode, "idle"))
        for scenario in ("settings", "modal"):
            results.append(run_case(args, f"{mode}-{scenario}", mode, scenario))
    results.append(run_case(args, "active-checkpoint", "attached", "active"))
    report = {
        "source": args.revision,
        "binary_sha256": args.binary_sha256,
        "runs": results,
        "passed": all(value["passed"] for value in results),
        "release_acceptance": False,
        "spoken_reader_qualification": False,
    }
    (args.evidence / "summary.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
