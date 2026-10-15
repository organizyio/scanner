#!/usr/bin/env python3
"""POSIX native CLI signal probe, with a FIFO barrier and bounded cleanup.

The FIFO proves hashing started after signal handlers were installed. Sending the
signal does not preempt active extraction: the writer then releases that read.
Windows console control events require a separate console-token qualification.
"""
import argparse
import errno
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import time


def run(binary, evidence):
    if os.name == "nt":
        raise RuntimeError("requires POSIX FIFO and signals; Windows console gate remains separate")
    binary, evidence = Path(binary).resolve(), Path(evidence).resolve()
    evidence.mkdir(parents=True, exist_ok=False)
    results = {}
    for name, native_signal in [("SIGINT", signal.SIGINT), ("SIGTERM", signal.SIGTERM)]:
        root = evidence / name
        root.mkdir()
        fifo, later, output = root / "a-barrier", root / "z-later.txt", evidence / (name + ".json")
        os.mkfifo(fifo)
        later.write_text("synthetic later record", encoding="utf-8")
        process = subprocess.Popen([str(binary), "fs", "-root", str(root), "-output", str(output)],
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        writer = None
        try:
            deadline = time.monotonic() + 10
            while writer is None and time.monotonic() < deadline and process.poll() is None:
                try:
                    writer = os.open(fifo, os.O_WRONLY | os.O_NONBLOCK)
                except OSError as error:
                    if error.errno != errno.ENXIO:
                        raise
                    time.sleep(0.01)
            if writer is None:
                raise RuntimeError("scanner never entered the synthetic read barrier")
            process.send_signal(native_signal)
            time.sleep(0.1)
            os.write(writer, b"synthetic FIFO content\n")
            os.close(writer)
            writer = None
            stdout, stderr = process.communicate(timeout=10)
            (evidence / (name + ".stderr.txt")).write_bytes(stderr)
            (evidence / (name + ".stdout.txt")).write_bytes(stdout)
            payload = json.loads(output.read_text(encoding="utf-8")) if output.exists() else {}
            files = payload.get("files", [])
            passed = process.returncode == 0 and len(files) == 1 and files[0].get("path") == str(fifo) and files[0].get("hash_status") == "ok"
            results[name] = {"passed": passed, "exit_code": process.returncode,
                             "file_count": len(files), "active_read_released": True}
        finally:
            if writer is not None:
                os.close(writer)
            if process.poll() is None:
                process.kill()
                process.communicate(timeout=5)
            fifo.unlink()
    report = {"schema": 1, "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
              "cases": results, "passed": all(v["passed"] for v in results.values()),
              "active_extraction_preemption": False, "release_acceptance": False}
    (evidence / "report.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    args = parser.parse_args()
    result = run(args.binary, args.evidence)
    print(json.dumps(result, indent=2))
    raise SystemExit(0 if result["passed"] else 1)
