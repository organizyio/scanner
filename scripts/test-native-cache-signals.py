#!/usr/bin/env python3
"""Test the registry emitter, with test-only instrumentation and a negative control."""

import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import tomllib

ROOT = Path(__file__).resolve().parents[1]
NATIVE = ROOT / "tools/project-catalog/native-a11y"


def packages(lock):
    result = {
        (p["name"], p["version"]): (p.get("source"), p.get("checksum"))
        for p in lock["package"]
    }
    if len(result) != len(lock["package"]):
        raise ValueError("Duplicate package origins in instrumentation graph")
    return result


def instrument(source, test, negative=False):
    needle = "                    &(body,),"
    if source.count(needle) != 1:
        raise ValueError("Reviewed emitter seam changed")
    if negative:
        source = source.replace(needle, "                    body,")
    return source + "\n" + test


def main():
    if sys.platform != "linux":
        raise SystemExit("This gate requires Linux and a private D-Bus session")
    registry = json.loads(
        subprocess.check_output(
            [
                "cargo",
                "+1.88",
                "metadata",
                "--locked",
                "--format-version",
                "1",
                "--manifest-path",
                str(NATIVE / "Cargo.toml"),
            ],
            text=True,
        )
    )
    package = next(p for p in registry["packages"] if p["name"] == "accesskit_unix")
    retirement = json.loads(
        (ROOT / "docs/follow-up/accesskit-vendor-retirement.json").read_text()
    )
    expected = retirement["replacement"]
    lock = tomllib.loads((NATIVE / "Cargo.lock").read_text())
    locked = next(p for p in lock["package"] if p["name"] == "accesskit_unix")
    if (
        any(
            locked.get(k) != expected[k]
            for k in ("name", "version", "source", "checksum")
        )
        or package["source"] != expected["source"]
    ):
        raise ValueError("Unreviewed cache-emitter package")
    original = Path(package["manifest_path"]).parent
    checksums = json.loads((original / ".cargo-checksum.json").read_text())
    if checksums["package"] != expected["checksum"]:
        raise ValueError("Registry archive checksum changed")
    for name, digest in checksums["files"].items():
        if hashlib.sha256((original / name).read_bytes()).hexdigest() != digest:
            raise ValueError("Registry source drift: " + name)
    production = (original / "src/atspi/bus.rs").read_text()
    test = (NATIVE / "tests/cache_wire.rs.inc").read_text()
    if hashlib.sha256(test.encode()).hexdigest() != retirement["wire_test_sha256"]:
        raise ValueError("Wire regression changed without review")
    before = (NATIVE / "Cargo.lock").read_bytes()
    with tempfile.TemporaryDirectory(prefix="scanner-cache-wire-") as temp:
        temp = Path(temp)
        source = temp / "accesskit_unix"
        shutil.copytree(original, source)
        project = temp / "native"
        project.mkdir()
        for name in ("Cargo.toml", "Cargo.lock"):
            shutil.copy2(NATIVE / name, project / name)
        shutil.copytree(NATIVE / "src", project / "src")
        config = temp / "patch.toml"
        config.write_text(
            "[patch.crates-io]\naccesskit_unix = { path = "
            + json.dumps(str(source))
            + " }\n"
        )
        cargo = ["cargo", "+1.88", "--config", str(config)]
        subprocess.run(
            cargo
            + [
                "update",
                "--offline",
                "--manifest-path",
                str(project / "Cargo.toml"),
                "-p",
                "accesskit_unix",
            ],
            check=True,
        )
        patched = packages(tomllib.loads((project / "Cargo.lock").read_text()))
        baseline = packages(lock)
        baseline[("accesskit_unix", expected["version"])] = (None, None)
        if patched != baseline:
            raise ValueError("Instrumentation changed the qualified dependency graph")
        for negative in (False, True):
            (source / "src/atspi/bus.rs").write_text(
                instrument(production, test, negative)
            )
            result = subprocess.run(
                ["dbus-run-session", "--"]
                + cargo
                + [
                    "test",
                    "--locked",
                    "--manifest-path",
                    str(project / "Cargo.toml"),
                    "--target-dir",
                    str(NATIVE / "target"),
                    "-p",
                    "accesskit_unix",
                    "cache_signals_preserve_struct_arguments",
                    "--",
                    "--ignored",
                ],
                cwd=ROOT,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                timeout=180,
            )
            print(result.stdout, end="")
            pattern = (
                r"test result: FAILED\. 0 passed; 1 failed; 0 ignored;"
                if negative
                else r"test result: ok\. 1 passed; 0 failed; 0 ignored;"
            )
            if (
                not re.search(pattern, result.stdout)
                or bool(result.returncode) != negative
            ):
                raise RuntimeError("Cache wire positive/negative control failed")
            print(
                "Expected flattened-emitter negative control rejected"
                if negative
                else "Published emitter positive control passed"
            )
    if (NATIVE / "Cargo.lock").read_bytes() != before:
        raise ValueError("Wire test changed production lock")


if __name__ == "__main__":
    main()
