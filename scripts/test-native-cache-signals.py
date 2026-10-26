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
import tarfile
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


def verify_registry_source(original, archive, expected):
    # Cargo caches do not consistently include vendor-only .cargo-checksum.json.
    # Verify the locked archive itself, then every extracted upstream file.
    if hashlib.sha256(archive.read_bytes()).hexdigest() != expected:
        raise ValueError("Registry archive checksum changed")
    names = set()
    with tarfile.open(archive, "r:gz") as contents:
        for member in contents.getmembers():
            path = Path(member.name)
            if (path.is_absolute() or ".." in path.parts or
                    not path.parts or path.parts[0] != original.name or
                    member.issym() or member.islnk()):
                raise ValueError("Unsafe registry archive path")
            if member.isdir():
                continue
            if not member.isfile() or member.size > 16 * 1024 * 1024:
                raise ValueError("Unexpected registry archive member")
            relative = Path(*path.parts[1:])
            if relative in names:
                raise ValueError("Duplicate registry archive member")
            names.add(relative)
            file = original / relative
            if file.is_symlink() or not file.is_file() or file.read_bytes() != contents.extractfile(member).read():
                raise ValueError("Registry source drift: " + str(relative))
    actual = {p.relative_to(original) for p in original.rglob("*") if p.is_file() or p.is_symlink()}
    if actual - names - {Path(".cargo-ok"), Path(".cargo-checksum.json")}:
        raise ValueError("Unexpected registry source files")


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
    archive = original.parents[2] / "cache" / original.parent.name / (original.name + ".crate")
    verify_registry_source(original, archive, expected["checksum"])
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
