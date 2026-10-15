#!/usr/bin/env python3
"""Send CTRL_C only to a newly created, owned Windows console.

Never broadcast to the SSH session or existing interactive desktop console.
A successful probe requires a valid partial inventory, not merely process exit.
"""
import argparse
import ctypes
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time


def require_console_members(actual, expected):
    if actual != expected:
        raise RuntimeError("Refusing control event: synthetic console membership changed")


def run(binary, evidence):
    if os.name != "nt":
        raise RuntimeError("Windows native console required")
    binary, evidence = Path(binary).resolve(), Path(evidence).resolve()
    evidence.mkdir(parents=True, exist_ok=False)
    root = evidence / "scan-root"
    root.mkdir()
    count = 4096
    for index in range(count):
        (root / ("fixture-%05d.txt" % index)).write_bytes(b"synthetic\n")
    output = evidence / "inventory.json"
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    kernel.SetConsoleCtrlHandler.argtypes = [ctypes.c_void_p, ctypes.c_int]
    kernel.GenerateConsoleCtrlEvent.argtypes = [ctypes.c_ulong, ctypes.c_ulong]
    def console_pids():
        values = (ctypes.c_ulong * 16)()
        count = kernel.GetConsoleProcessList(values, len(values))
        if not count or count > len(values):
            raise RuntimeError("Cannot verify exclusive ownership of synthetic console")
        return set(values[:count])

    require_console_members(console_pids(), {os.getpid()})
    # This disposable worker owns a new console. Clear an inherited SSH ignore
    # flag before launching Scanner, then ignore the event only in this worker.
    if not kernel.SetConsoleCtrlHandler(None, False):
        raise ctypes.WinError(ctypes.get_last_error())
    process = subprocess.Popen([str(binary), "fs", "-root", str(root), "-output", str(output)],
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    try:
        if not kernel.SetConsoleCtrlHandler(None, True):
            raise ctypes.WinError(ctypes.get_last_error())
        time.sleep(0.03)
        if process.poll() is not None or output.exists():
            raise RuntimeError("fixture completed before control event; no interruption qualified")
        require_console_members(console_pids(), {os.getpid(), process.pid})
        if not kernel.GenerateConsoleCtrlEvent(0, 0):
            raise ctypes.WinError(ctypes.get_last_error())
        stdout, stderr = process.communicate(timeout=20)
        (evidence / "stdout.txt").write_bytes(stdout)
        (evidence / "stderr.txt").write_bytes(stderr)
        payload = json.loads(output.read_text(encoding="utf-8")) if output.exists() else {}
        records = payload.get("files", [])
        passed = process.returncode == 0 and isinstance(payload.get("folders"), dict) and 0 < len(records) < count and all(r.get("hash_status") == "ok" for r in records)
        report = {"schema": 1, "event": "CTRL_C_EVENT", "owned_new_console": True,
                  "exit_code": process.returncode, "fixture_files": count,
                  "partial_records": len(records), "passed": passed,
                  "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
                  "release_acceptance": False}
        (evidence / "report.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
        return report
    finally:
        if process.poll() is None:
            process.kill()
            process.communicate(timeout=5)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    parser.add_argument("--console-worker", action="store_true", help=argparse.SUPPRESS)
    args = parser.parse_args()
    if not args.console_worker:
        if os.name != "nt":
            parser.error("Windows native console required")
        child = subprocess.run([sys.executable, str(Path(__file__).resolve()),
                                "--binary", str(args.binary), "--evidence", str(args.evidence),
                                "--console-worker"], creationflags=subprocess.CREATE_NEW_CONSOLE, timeout=60)
        raise SystemExit(child.returncode)
    report = run(args.binary, args.evidence)
    print(json.dumps(report, indent=2))
    raise SystemExit(0 if report["passed"] else 1)
