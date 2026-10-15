#!/usr/bin/env python3
"""Synthetic native qualification. Evidence belongs outside the source checkout.

This runner never asserts GUI qualification from compilation or fixture creation.
Run `prepare` before native interaction; run `cli` with an actual catalog binary.
"""
import argparse
import contextlib
import ctypes
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys
import time


def normalized_path(value):
    return re.sub(r"/+", "/", str(value).replace("\\", "/")).casefold()


def assess_denial(name, result, payload, stderr, affected, verified, windows=None):
    """Require this fixture's actual diagnostic, successful partial envelope,
    and an affected read-error record for sharing violations. Unrelated stderr
    and unsuccessful CLI execution can never satisfy a native qualification.
    """
    windows = os.name == "nt" if windows is None else windows
    diagnostic = normalized_path(affected) in normalized_path(stderr)
    affected_error = bool(payload) and any(
        normalized_path(v.get("path", "")) == normalized_path(affected)
        and v.get("hash_status") == "error" for v in payload.get("files", []))
    result.update(actual_denial_verified=verified, associated_diagnostic=diagnostic,
                  affected_hash_error=affected_error)
    if result["exit_code"] != 0 or not result["valid_envelope"]:
        result["status"] = "failed"
    elif not verified:
        result["status"] = "pending-native-read-denial" if windows else "failed"
    elif diagnostic and (name == "access-denied" or affected_error):
        result["status"] = "passed"
    elif windows and not affected_error and not diagnostic:
        result["status"] = "pending-native-read-denial"
    else:
        result["status"] = "failed"
    return result


def isolated_environment(root):
    root = Path(root).resolve()
    env = os.environ.copy()
    for key, name in {
        "HOME": "home", "USERPROFILE": "home", "APPDATA": "config",
        "LOCALAPPDATA": "local", "XDG_CONFIG_HOME": "config",
        "XDG_CACHE_HOME": "cache", "XDG_DATA_HOME": "data",
    }.items():
        directory = root / name
        directory.mkdir(parents=True, exist_ok=True)
        env[key] = str(directory)
    return env


def settings_directory(root):
    root = Path(root)
    if sys.platform == "darwin":
        return root / "home" / "Library" / "Application Support" / "ScannerApp"
    return root / "config" / "ScannerApp"


def prepare(root, files=3):
    root = Path(root).resolve()
    checkout = Path(__file__).resolve().parents[1]
    if (checkout / "Cargo.toml").is_file() and root.is_relative_to(checkout):
        raise ValueError("private qualification evidence must be outside the checkout")
    if root.exists() and any(root.iterdir()):
        raise ValueError("use an empty fixture directory; existing content is never overwritten")
    root.mkdir(parents=True, exist_ok=True)
    # Refuse overwriting another run's settings or evidence.
    marker = root / "qualification-fixture.json"
    if marker.exists():
        raise ValueError("use a fresh fixture directory for each qualification run")
    env = isolated_environment(root)
    scan = root / "scan-root"
    scan.mkdir()
    (scan / "Unicode-Δ-日本語").mkdir()
    (scan / "Unicode-Δ-日本語" / "hello.txt").write_text("synthetic Unicode fixture\n")
    (scan / "go.mod").write_text("module qualification.invalid/synthetic\n\ngo 1.23\n")
    for index in range(files):
        (scan / ("file-%06d.txt" % index)).write_bytes(b"synthetic qualification\n" * 256)
    symlink = "unsupported"
    try:
        (scan / "linked-file.txt").symlink_to("file-000000.txt")
        symlink = "created"
    except (OSError, NotImplementedError):
        pass
    cfg = settings_directory(root)
    cfg.mkdir(parents=True)
    # JSON quoted scalars are YAML compatible, including Windows path escaping.
    settings = """active_profile: Qualification
profiles:
  Qualification:
    name: Qualification
    scan_roots:
      - path: %s
        enabled: true
    exclude_dirs: [".git"]
    max_file_size_mb: 0
    hash_files: true
    hash_max_file_size_mb: 200
    project_detection: true
    checkpoint_enabled: true
    checkpoint_path: %s
""" % (json.dumps(str(scan)), json.dumps(str(root / "checkpoint.json")))
    (cfg / "settings.yaml").write_text(settings)
    data = {"schema": 1, "system": platform.system(), "architecture": platform.machine(),
            "files": files, "symlink": symlink, "gui_qualification": "pending",
            "release_acceptance": False}
    marker.write_text(json.dumps(data, indent=2) + "\n")
    return env


@contextlib.contextmanager
def denied_directory(path):
    """Deny only a newly created synthetic directory, restore in every exit path."""
    path = Path(path).resolve()
    path.mkdir()
    (path / "private-fixture.txt").write_text("synthetic denied content")
    if os.name == "nt":
        who = subprocess.check_output(["whoami", "/user", "/fo", "csv", "/nh"], text=True)
        import csv
        sid = next(csv.reader([who.strip()]))[1]
        # Explicit deny on this directory only; no inherited/user-profile ACL changes.
        # Deny data/list access only. Generic R also denies READ_CONTROL, which
        # icacls needs to remove the temporary ACE under ordinary CI tokens.
        subprocess.run(["icacls", str(path), "/deny", "*" + sid + ":(OI)(CI)(RD)"],
                       check=True, capture_output=True)
        restore = lambda: subprocess.run(["icacls", str(path), "/remove:d", "*" + sid],
                                         check=True, capture_output=True)
    else:
        mode = path.stat().st_mode & 0o777
        path.chmod(0)
        restore = lambda: path.chmod(mode)
    try:
        try:
            list(path.iterdir())
            (path / "private-fixture.txt").read_bytes()
        except PermissionError:
            yield True
        else:
            yield False
    finally:
        restore()


@contextlib.contextmanager
def exclusive_file(path):
    """Actual Windows sharing denial. POSIX advisory locks do not imply denial."""
    if os.name != "nt":
        yield False
        return
    from ctypes import wintypes
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    kernel.CreateFileW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD,
                                  ctypes.c_void_p, wintypes.DWORD, wintypes.DWORD, wintypes.HANDLE]
    kernel.CreateFileW.restype = wintypes.HANDLE
    kernel.CloseHandle.argtypes = [wintypes.HANDLE]
    handle = kernel.CreateFileW(str(path), 0x80000000, 0, None, 3, 0x80, None)
    if handle == ctypes.c_void_p(-1).value:
        raise ctypes.WinError(ctypes.get_last_error())
    try:
        try:
            Path(path).read_bytes()
        except PermissionError:
            yield True
        else:
            yield False
    finally:
        kernel.CloseHandle(handle)


def cli(binary, root):
    root = Path(root).resolve()
    if not (root / "qualification-fixture.json").exists():
        raise ValueError("prepare a fresh fixture before CLI qualification")
    env = isolated_environment(root)
    scan = root / "scan-root"
    results = {"schema": 1, "system": platform.system(), "architecture": platform.machine(),
               "binary_sha256": hashlib.sha256(Path(binary).read_bytes()).hexdigest(),
               "release_acceptance": False, "gui_qualification": "pending", "cases": {}}
    observations = {}

    def run(name):
        output = root / (name + ".json")
        started = time.monotonic()
        p = subprocess.run([str(Path(binary).resolve()), "fs", "-root", str(scan),
                            "-output", str(output)], env=env, capture_output=True,
                           text=True, timeout=120)
        (root / (name + ".stderr.txt")).write_text(p.stderr)
        payload = json.loads(output.read_text(encoding="utf-8")) if output.exists() else None
        value = {"exit_code": p.returncode, "elapsed_seconds": round(time.monotonic()-started, 4),
                 "valid_envelope": isinstance(payload, dict) and
                 isinstance(payload.get("files"), list) and isinstance(payload.get("folders"), dict),
                 "diagnostic_present": bool(p.stderr.strip())}
        value["hash_error_count"] = sum(v.get("hash_status") == "error" for v in payload.get("files", [])) if payload else 0
        if name == "unicode-symlink":
            value["unicode_record_present"] = bool(payload) and any(
                "Unicode-Δ-日本語" in record.get("path", "") for record in payload.get("files", []))
            value["status"] = "passed" if value["valid_envelope"] and value["unicode_record_present"] and p.returncode == 0 else "failed"
        results["cases"][name] = value
        observations[name] = (payload, p.stderr)
        return value

    run("unicode-symlink")
    with denied_directory(scan / "denied-fixture") as denied:
        value = run("access-denied")
        assess_denial("access-denied", value, *observations["access-denied"], scan / "denied-fixture", denied)
    with exclusive_file(scan / "file-000000.txt") as locked:
        value = run("locked-file")
        if os.name == "nt":
            assess_denial("locked-file", value, *observations["locked-file"], scan / "file-000000.txt", locked)
        else:
            value.update(actual_denial_verified=False, status="not-applicable-posix-advisory-locks")
    (root / "cli-report.json").write_text(json.dumps(results, indent=2) + "\n")
    return results


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["prepare", "cli", "launch"])
    parser.add_argument("--fixture", type=Path, required=True)
    parser.add_argument("--binary", type=Path)
    parser.add_argument("--files", type=int, default=3)
    args = parser.parse_args()
    if args.files < 1:
        parser.error("--files must be positive")
    if args.command == "prepare":
        prepare(args.fixture, args.files)
    else:
        if not args.binary or not args.binary.is_file():
            parser.error("--binary must identify the built native executable")
        if args.command == "cli":
            result = cli(args.binary, args.fixture)
            print(json.dumps(result, indent=2))
            return int(any(v.get("status", "").startswith(("failed", "pending")) for v in result["cases"].values()))
        if not (args.fixture / "qualification-fixture.json").is_file():
            parser.error("prepare before launch; never launch with default personal settings")
        return subprocess.call([str(args.binary.resolve())], env=isolated_environment(args.fixture))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
