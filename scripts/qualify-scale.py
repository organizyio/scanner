#!/usr/bin/env python3
"""Source-build a synthetic scale probe; private results never approve releases."""

import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import platform
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
MODES = (
    "fast",
    "slow_callback",
    "slow_output",
    "cancel",
    "callback_error",
    "output_error",
)


def run_probe(command, stdout, stderr, timeout):
    """Per-child RSS via wait4 on POSIX; unsupported platforms report null, never zero."""
    with stdout.open("xb") as out, stderr.open("xb") as err:
        process = subprocess.Popen(command, stdout=out, stderr=err)
        started = time.monotonic()
        usage = None
        try:
            while True:
                if hasattr(os, "wait4"):
                    pid, status, resource = os.wait4(process.pid, os.WNOHANG)
                    if pid:
                        process.returncode = os.waitstatus_to_exitcode(status)
                        usage = resource
                        break
                elif process.poll() is not None:
                    break
                if time.monotonic() - started >= timeout:
                    raise TimeoutError("Scale probe exceeded owned-process deadline")
                time.sleep(0.005)
        finally:
            if process.returncode is None:
                process.kill()
                process.wait()
    if process.returncode:
        raise RuntimeError("Scale probe failed; inspect private stderr log")
    if stdout.stat().st_size > 65536:
        raise ValueError("Probe report exceeds limit")
    value = json.loads(stdout.read_text())
    value["peak_rss_bytes"] = (
        (int(usage.ru_maxrss) * (1 if platform.system() == "Darwin" else 1024))
        if usage
        else None
    )
    value["rss_scope"] = (
        "Per-process high-water RSS, including post-stop recovery"
        if usage
        else "Unavailable: host has no wait4 resource reporting"
    )
    return value


def validate(value, mode, walk, files, stop):
    if (
        value.get("passed") is not True
        or value.get("mode") != mode
        or value.get("walk_mode") != walk
        or value.get("fixture_files") != files
    ):
        raise ValueError("Probe identity/result mismatch")
    partial = mode in ("cancel", "callback_error", "output_error")
    count = stop if partial else files
    if (
        value.get("files_delivered") != count
        or value.get("bytes_delivered") != count * 256
    ):
        raise ValueError("Incorrect delivered inventory")
    if (
        value.get("expected_failure_observed") is not mode.endswith("error")
        or value.get("recovery_inventory_verified") is not partial
    ):
        raise ValueError("Missing error/recovery proof")
    for key in ["elapsed_ms", "first_record_ms"] + (
        ["cancel_return_ms"] if mode == "cancel" else []
    ):
        n = value.get(key)
        if type(n) not in (float, int) or not math.isfinite(n) or n < 0:
            raise ValueError("Missing/invalid timing")
    if (
        value["first_record_ms"] > value["elapsed_ms"]
        or value.get("release_acceptance") is not False
    ):
        raise ValueError("Inconsistent timing/acceptance")


def fixture(root, files):
    for i in range(files):
        folder = root / ("alpha" if i % 2 == 0 else "unicode-β") / f"shard-{i % 32:02}"
        folder.mkdir(parents=True, exist_ok=True)
        (folder / f"file-{i}.txt").write_bytes(b"x" * 256)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--output",
        type=Path,
        required=True,
        help="New private directory outside checkout",
    )
    parser.add_argument("--files", type=int, nargs="+", default=[4096, 32768])
    parser.add_argument("--repeats", type=int, default=3)
    parser.add_argument("--cancel-repeats", type=int, default=8)
    parser.add_argument("--delay-us", type=int, default=25)
    parser.add_argument("--timeout", type=float, default=120)
    args = parser.parse_args()
    out = args.output.resolve()
    if (
        out.is_relative_to(ROOT)
        or any(n < 2 or n > 1000000 for n in args.files)
        or not 1 <= args.repeats <= 20
        or not 1 <= args.cancel_repeats <= 100
        or not 1 <= args.delay_us <= 10000
        or not 0 < args.timeout <= 1800
    ):
        parser.error("Invalid limits or output inside checkout")
    revision = subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=ROOT, text=True
    ).strip()
    if subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT):
        parser.error(
            "Use a clean committed Scanner checkout for source-bound measurements"
        )
    out.mkdir(parents=True, exist_ok=False)
    with (out / "build.log").open("x") as log:
        subprocess.run(
            [
                "cargo",
                "+1.88",
                "build",
                "--locked",
                "--release",
                "-p",
                "scanner-engine",
                "--example",
                "scale_probe",
                "--target-dir",
                str(out / "build"),
            ],
            cwd=ROOT,
            stdout=log,
            stderr=subprocess.STDOUT,
            check=True,
            timeout=1800,
        )
    binary = (
        out
        / "build/release/examples"
        / ("scale_probe.exe" if os.name == "nt" else "scale_probe")
    )
    report = {
        "schema": 1,
        "source_revision": revision,
        "probe_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
        "platform": platform.system() + "-" + platform.machine(),
        "rustc": subprocess.check_output(
            ["rustc", "+1.88", "--version"], text=True
        ).strip(),
        "fixture": {
            "payload_bytes": 256,
            "payload_sha256": hashlib.sha256(b"x" * 256).hexdigest(),
            "roots": 2,
            "shards": 32,
            "file_names": "file-N.txt; even IDs alpha, odd IDs unicode-β",
        },
        "delay_us": args.delay_us,
        "repeats": args.repeats,
        "cancel_repeats": args.cancel_repeats,
        "timeout_seconds": args.timeout,
        "runs": [],
        "passed": False,
        "interpretation": "Synthetic observations, not improvement claims or a worst-case blocking-I/O bound. RSS includes probe/runtime and recovery; queue bounds are proved separately by deterministic tests.",
        "macos_arm64_retina_qualified": False,
        "runtime_convergence": False,
        "production_pin_adoption": False,
        "release_acceptance": False,
    }
    try:
        for files in args.files:
            with tempfile.TemporaryDirectory(prefix="fixture-", dir=out) as folder:
                fixture(Path(folder), files)
                stop = min(128, files // 2)
                for walk in ("standard", "full"):
                    for mode in MODES:
                        for repeat in range(
                            args.cancel_repeats if mode == "cancel" else args.repeats
                        ):
                            label = f"{files}-{walk}-{mode}-{repeat + 1}"
                            value = run_probe(
                                [
                                    str(binary),
                                    folder,
                                    mode,
                                    str(files),
                                    str(args.delay_us),
                                    str(stop),
                                    walk,
                                ],
                                out / (label + ".stdout.json"),
                                out / (label + ".stderr.log"),
                                args.timeout,
                            )
                            validate(value, mode, walk, files, stop)
                            value["repeat"] = repeat + 1
                            report["runs"].append(value)
                            print(label, "passed", flush=True)
        report["passed"] = True
    finally:
        (out / "report.json").write_text(json.dumps(report, indent=2) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
