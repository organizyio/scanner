#!/usr/bin/env python3
"""Source-pinned Scanner Apple Silicon qualification and evidence packaging.

Use --help. All GUI roots/settings are synthetic and outside the fresh clone.
No existing checkout edits, pushes, releases, spoken-reader tests or personal scans are run.
"""

import argparse
import datetime as dt
import hashlib
import importlib.util
import json
import os
import platform
import re
import signal
import stat
import struct
import subprocess
import sys
import tarfile
import time
import uuid
import zipfile
from pathlib import Path

REVISION = "@SCANNER_REVISION@"
ORIGIN = "https://github.com/organizyio/scanner.git"
GO_VERSION = "1.24.13"
GO_ARCHIVE = "go1.24.13.darwin-arm64.tar.gz"
GO_SHA256 = "f282d882c3353485e2fc6c634606d85caf36e855167d59b996dbeae19fa7629a"
KIT = Path(__file__).resolve().parent


def stamp():
    return dt.datetime.now(dt.timezone.utc).isoformat()


def save(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")
    temporary.replace(path)


def read(path):
    return json.loads(path.read_text())


def digest(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def output(command, cwd=None, env=None):
    return subprocess.check_output(command, cwd=cwd, env=env, text=True).strip()


def native_host():
    if sys.platform != "darwin" or platform.machine() != "arm64" or os.geteuid() == 0:
        raise RuntimeError(
            "Use an unprivileged native arm64 Python on an Apple Silicon Mac; no sudo/Rosetta"
        )
    translated = subprocess.run(
        ["sysctl", "-n", "sysctl.proc_translated"], capture_output=True, text=True
    )
    if translated.returncode == 0 and translated.stdout.strip() == "1":
        raise RuntimeError("Rosetta process detected; reopen a native terminal/Python")


def load(workspace):
    root = workspace.expanduser().resolve()
    config = read(root / "run.json")
    if config["revision"] != REVISION or config["origin"] != ORIGIN:
        raise RuntimeError("Workspace does not match this pinned kit")
    return root


def environment(root):
    env = dict(os.environ)
    for key in list(env):
        if key.startswith("CGO_") or key in {
            "GOOS",
            "GOARCH",
            "GOROOT",
            "GOFLAGS",
            "GOWORK",
            "RUSTFLAGS",
            "RUSTUP_TOOLCHAIN",
            "CARGO_TARGET_DIR",
            "CARGO_BUILD_TARGET",
            "CARGO_ENCODED_RUSTFLAGS",
            "RUSTC_WRAPPER",
            "RUSTC_WORKSPACE_WRAPPER",
            "FYNE_SCALE",
        }:
            env.pop(key, None)
    env.update(
        {
            "PATH": str(root / "tools/go/bin")
            + os.pathsep
            + str(root / "tools/cargo/bin")
            + os.pathsep
            + env["PATH"],
            "RUSTUP_HOME": str(root / "tools/rustup"),
            "CARGO_HOME": str(root / "tools/cargo"),
            "CARGO_BUILD_JOBS": "2",
            "GOTOOLCHAIN": "local",
            "GOENV": "off",
            "GOWORK": "off",
            "GOPATH": str(root / "tools/gopath"),
            "GOMODCACHE": str(root / "tools/gopath/pkg/mod"),
            "GOCACHE": str(root / "tools/go-build-cache"),
            "GOPROXY": "https://proxy.golang.org,direct",
            "GOSUMDB": "sum.golang.org",
            "GOPRIVATE": "",
            "GOOS": "darwin",
            "GOARCH": "arm64",
            "GOFLAGS": "-mod=readonly",
            "CGO_ENABLED": "1",
            "CC": "/usr/bin/clang",
            "CXX": "/usr/bin/clang++",
            "TMPDIR": str(root / "tmp"),
        }
    )
    return env


def journal(root, value):
    path = root / "evidence/commands.json"
    values = read(path) if path.exists() else []
    values.append(value)
    save(path, values)


def run(root, name, command, cwd=None, env=None, timeout=1800, check=True):
    label = re.sub(r"[^a-zA-Z0-9_-]", "-", name)
    log = root / "evidence/logs" / (label + "-" + uuid.uuid4().hex[:8] + ".log")
    start = time.monotonic()
    record = {
        "name": name,
        "started_utc": stamp(),
        "command": list(map(str, command)),
        "cwd": str(cwd or root),
        "log": str(log.relative_to(root / "evidence")),
    }
    print("RUN:", name, "LOG:", log, flush=True)
    with log.open("w") as stream:
        process = subprocess.Popen(
            list(map(str, command)),
            cwd=cwd or root,
            env=env or environment(root),
            stdout=stream,
            stderr=subprocess.STDOUT,
            start_new_session=True,
        )
        try:
            code = process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            record["timed_out"] = True
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait(timeout=5)
            code = 124
    record.update(
        exit_code=code,
        elapsed_seconds=round(time.monotonic() - start, 3),
        finished_utc=stamp(),
    )
    journal(root, record)
    if code and check:
        raise RuntimeError(f"{name} failed (exit {code}); retain and inspect {log}")
    return code


def check_source(root):
    source = root / "source"
    if output(["git", "rev-parse", "HEAD"], source) != REVISION:
        raise RuntimeError(
            "Source revision changed; no qualification against unreviewed source"
        )
    if output(["git", "remote", "get-url", "origin"], source) != ORIGIN:
        raise RuntimeError("Unexpected source remote")
    status = output(
        ["git", "status", "--porcelain=v1", "--untracked-files=all"], source
    )
    save(
        root / "evidence/source-status.json",
        {"revision": REVISION, "status": status, "checked_utc": stamp()},
    )
    if status:
        raise RuntimeError(
            "Source is dirty; preserve the changes and report, do not reset or hide them"
        )


def initialize(path):
    native_host()
    root = path.expanduser().resolve()
    if root.exists() and any(root.iterdir()):
        raise RuntimeError(
            "Use a completely new empty workspace; existing content is never overwritten"
        )
    for name in ["evidence/logs", "evidence/reports", "tools", "bin", "tmp", "share"]:
        (root / name).mkdir(parents=True, exist_ok=True)
    save(
        root / "run.json",
        {
            "revision": REVISION,
            "origin": ORIGIN,
            "created_utc": stamp(),
            "workspace_id": uuid.uuid4().hex,
            "python": sys.executable,
        },
    )
    run(
        root,
        "clone-public-source",
        ["git", "clone", "--no-checkout", ORIGIN, root / "source"],
        env=dict(os.environ),
    )
    run(
        root,
        "checkout-reviewed-commit",
        ["git", "checkout", "--detach", REVISION],
        cwd=root / "source",
        env=dict(os.environ),
    )
    host = {
        "recorded_utc": stamp(),
        "python": sys.version,
        "python_machine": platform.machine(),
        "os": output(["sw_vers"]),
        "machine": output(["uname", "-m"]),
        "model": output(["sysctl", "-n", "hw.model"]),
        "root_user": False,
        "serial_numbers_collected": False,
    }
    save(root / "evidence/host.json", host)
    save(
        root / "evidence/operator-report.json",
        {
            "cases": {},
            "release_acceptance": False,
            "instructions": "Record actual observations with the record subcommand; missing evidence stays pending.",
        },
    )
    check_source(root)
    print("Workspace:", root)


def setup(root):
    native_host()
    check_source(root)
    env = environment(root)
    run(root, "check-apple-compiler", ["xcrun", "--find", "clang"], env=env)
    run(root, "check-swift-compiler", ["xcrun", "--find", "swiftc"], env=env)
    archive = root / "tools" / GO_ARCHIVE
    if not archive.exists():
        run(
            root,
            "download-pinned-go",
            [
                "curl",
                "--fail",
                "--location",
                "--silent",
                "--show-error",
                "--proto",
                "=https",
                "--tlsv1.2",
                "https://go.dev/dl/" + GO_ARCHIVE,
                "-o",
                archive,
            ],
            env=env,
        )
    if digest(archive) != GO_SHA256:
        raise RuntimeError("Go download checksum mismatch; do not execute it")
    if not (root / "tools/go").exists():
        with tarfile.open(archive) as tar:
            for item in tar.getmembers():
                if item.name != "go" and not item.name.startswith("go/"):
                    raise RuntimeError("Unexpected Go archive path")
            tar.extractall(root / "tools", filter="data")
    if not (root / "tools/cargo/bin/rustup").exists():
        installer = root / "tools/rustup-install.sh"
        run(
            root,
            "download-rustup-installer",
            [
                "curl",
                "--fail",
                "--location",
                "--silent",
                "--show-error",
                "--proto",
                "=https",
                "--tlsv1.2",
                "https://sh.rustup.rs",
                "-o",
                installer,
            ],
            env=env,
        )
        save(
            root / "evidence/rustup-installer.json",
            {"url": "https://sh.rustup.rs", "sha256": digest(installer)},
        )
        run(
            root,
            "install-isolated-rustup",
            [
                "sh",
                installer,
                "-y",
                "--no-modify-path",
                "--profile",
                "minimal",
                "--default-host",
                "aarch64-apple-darwin",
                "--default-toolchain",
                "none",
            ],
            env=env,
        )
    run(
        root,
        "install-pinned-rust",
        [
            "rustup",
            "toolchain",
            "install",
            "1.88",
            "--profile",
            "minimal",
            "--component",
            "clippy",
            "--component",
            "rustfmt",
            "--no-self-update",
        ],
        env=env,
    )
    go = output(["go", "version"], env=env)
    rust = output(["rustc", "+1.88", "-vV"], env=env)
    if (
        go != "go version go1.24.13 darwin/arm64"
        or "host: aarch64-apple-darwin" not in rust
    ):
        raise RuntimeError("Native pinned compiler architecture/version mismatch")
    save(
        root / "evidence/toolchains.json",
        {
            "go": go,
            "rust": rust,
            "swift": output(["swiftc", "--version"], env=env),
            "go_archive_sha256": GO_SHA256,
            "isolated_toolchains": True,
        },
    )


def build(root):
    native_host()
    check_source(root)
    source, env = root / "source", environment(root)
    run(
        root,
        "native-adapter-check-build",
        [sys.executable, source / "scripts/build-native-a11y.py", "--check"],
        source,
        env,
    )
    module = source / "tools/project-catalog"
    build_id = "scanner-qualification-" + REVISION + "-darwin-arm64"
    run(
        root,
        "build-native-gui",
        [
            "go",
            "build",
            "-a",
            "-mod=readonly",
            "-ldflags=-buildid=" + build_id,
            "-o",
            root / "bin/scan-gui",
            "./cmd/scan-gui",
        ],
        module,
        env,
    )
    run(
        root,
        "build-native-cli",
        ["go", "build", "-mod=readonly", "-o", root / "bin/scan-cli", "./cmd/scan-cli"],
        module,
        env,
    )
    for src, name in [
        (source / "scripts/platform-macos.swift", "ax-helper"),
        (source / "scripts/platform-macos-observer.swift", "ax-observer"),
        (KIT / "display-probe.swift", "display-probe"),
    ]:
        run(
            root, "compile-" + name, ["swiftc", src, "-o", root / "bin" / name], env=env
        )
    files = {}
    for name in ["scan-gui", "scan-cli", "ax-helper", "ax-observer", "display-probe"]:
        binary = root / "bin" / name
        arch = output(["lipo", "-archs", binary], env=env)
        if arch != "arm64":
            raise RuntimeError(
                "Binary must contain only native arm64: " + name + " " + arch
            )
        files[name] = {"sha256": digest(binary), "architecture": arch}
    save(
        root / "evidence/build.json",
        {"source": REVISION, "binaries": files, "build_id": build_id},
    )
    run(
        root,
        "gui-go-build-info",
        ["go", "version", "-m", root / "bin/scan-gui"],
        env=env,
    )
    run(
        root,
        "display-host-metadata",
        [root / "bin/display-probe", "--host", root / "evidence/displays.json"],
        env=env,
    )
    check_source(root)


def permissions(root, request):
    command = [
        root / "bin/display-probe",
        "--permissions",
        root / "evidence/permissions.json",
    ]
    if request:
        command.append("--request")
    run(
        root,
        "permissions-request" if request else "permissions-check",
        command,
        timeout=60,
    )


def checks(root):
    native_host()
    check_source(root)
    source, env = root / "source", environment(root)
    jobs = [
        ("preferred-rust-checks", ["make", "check"], source),
        (
            "qualification-regressions",
            [
                sys.executable,
                "-m",
                "unittest",
                "discover",
                "-s",
                "scripts",
                "-p",
                "test_*.py",
            ],
            source,
        ),
        (
            "catalog-vet",
            ["go", "vet", "-mod=readonly", "./..."],
            source / "tools/project-catalog",
        ),
        (
            "catalog-tests",
            ["go", "test", "-mod=readonly", "./...", "-count=1"],
            source / "tools/project-catalog",
        ),
        (
            "gui-native-race",
            [
                "go",
                "test",
                "-mod=readonly",
                "-race",
                "-tags",
                "ci",
                "./internal/gui",
                "./internal/nativea11y",
            ],
            source / "tools/project-catalog",
        ),
        (
            "fyne-queue-race",
            ["go", "test", "-mod=readonly", "-race", "./internal/async"],
            source / "tools/project-catalog/third_party/fyne",
        ),
        (
            "native-owner-lifecycle-build",
            [
                "clang",
                "-std=c11",
                "-fblocks",
                "tools/project-catalog/native-a11y/tests/macos_lifecycle.m",
                "-framework",
                "AppKit",
                "-framework",
                "Foundation",
                "-o",
                root / "bin/native-lifecycle",
            ],
            source,
        ),
        ("native-owner-lifecycle-run", [root / "bin/native-lifecycle"], source),
        (
            "compat-engine-fmt",
            ["cargo", "+1.88", "fmt", "--all", "--", "--check"],
            source / "compat/scan-engine",
        ),
        (
            "compat-engine-clippy",
            [
                "cargo",
                "+1.88",
                "clippy",
                "--workspace",
                "--all-targets",
                "--locked",
                "--",
                "-D",
                "warnings",
            ],
            source / "compat/scan-engine",
        ),
        (
            "compat-engine-tests",
            ["cargo", "+1.88", "test", "--workspace", "--locked"],
            source / "compat/scan-engine",
        ),
        (
            "build-local-engine",
            [sys.executable, "scripts/build-components.py", "engine", "--archive"],
            source,
        ),
        (
            "build-embedded-wrapper",
            [sys.executable, "scripts/build-components.py", "wrapper", "--archive"],
            source,
        ),
        (
            "wrapper-vet",
            ["go", "vet", "-mod=readonly", "./..."],
            source / "compat/scan-fs",
        ),
        (
            "wrapper-unit",
            ["go", "test", "-mod=readonly", "./...", "-count=1"],
            source / "compat/scan-fs",
        ),
        (
            "wrapper-real-engine-integration",
            [
                "go",
                "test",
                "-mod=readonly",
                "-tags=integration",
                "./tests/integration/...",
                "-count=1",
            ],
            source / "compat/scan-fs",
        ),
        (
            "public-history-source-contracts",
            [sys.executable, "scripts/verify-consolidation.py"],
            source,
        ),
    ]
    results = {}
    for name, command, cwd in jobs:
        try:
            job_env = (
                dict(env, CGO_ENABLED="0")
                if name in {"catalog-vet", "catalog-tests"}
                else env
            )
            code = run(root, name, command, cwd, job_env, check=False)
        except OSError as error:
            code = 127
            journal(
                root,
                {
                    "name": name,
                    "exit_code": code,
                    "error": str(error),
                    "finished_utc": stamp(),
                },
            )
        results[name] = {"exit_code": code, "passed": code == 0}
    for name, script in [
        ("filesystem", "platform-catalog-native.py"),
        ("signals", "platform-catalog-signal.py"),
    ]:
        path = root / "evidence" / name
        command = [sys.executable, source / "scripts" / script, "--evidence", path]
        command += (
            ["--source", source]
            if name == "filesystem"
            else ["--binary", root / "bin/scan-cli"]
        )
        code = run(root, name, command, source, env, check=False)
        results[name] = {"exit_code": code, "passed": code == 0}
    fixture = root / "evidence/cli"
    run(
        root,
        "prepare-cli-fixture",
        [
            sys.executable,
            source / "scripts/platform-qualification.py",
            "prepare",
            "--fixture",
            fixture,
        ],
        env=env,
    )
    code = run(
        root,
        "cli-native-permission-cases",
        [
            sys.executable,
            source / "scripts/platform-qualification.py",
            "cli",
            "--fixture",
            fixture,
            "--binary",
            root / "bin/scan-cli",
        ],
        env=env,
        check=False,
    )
    results["cli-native-permission-cases"] = {"exit_code": code, "passed": code == 0}
    save(
        root / "evidence/checks-summary.json",
        {"checks": results, "passed": all(v["passed"] for v in results.values())},
    )
    check_source(root)
    if not all(v["passed"] for v in results.values()):
        raise RuntimeError(
            "Some checks failed; preserve all results and inspect checks-summary.json"
        )


def validate_label(value):
    if not re.fullmatch(r"[a-z][a-z0-9_-]{0,63}", value):
        raise RuntimeError(
            "Label must contain lowercase letters/numbers/hyphens/underscores and start with a letter"
        )
    return value


def automated(root, label):
    native_host()
    check_source(root)
    label = validate_label(label)
    base = root / "evidence/automated" / label
    base.mkdir(parents=True, exist_ok=False)
    source, env = root / "source", environment(root)
    run(
        root, "serial-ax-latency-" + label,
        [sys.executable, source / "scripts/qualify-macos-ax-latency.py",
         "--binary", root / "bin/scan-gui", "--helper", root / "bin/ax-helper",
         "--evidence", base / "ax-latency", "--revision", REVISION],
        source, env, timeout=60,
    )
    run(
        root,
        "native-full-flow-" + label,
        [
            sys.executable,
            source / "scripts/platform-macos-session.py",
            "--fixture",
            base / "flow",
            "--binary",
            root / "bin/scan-gui",
            "--helper",
            root / "bin/ax-helper",
            "--revision",
            REVISION,
        ],
        source,
        env,
        timeout=900,
    )
    run(
        root,
        "native-shutdown-matrix-" + label,
        [
            sys.executable,
            source / "scripts/qualify-macos-shutdown.py",
            "--binary",
            root / "bin/scan-gui",
            "--helper",
            root / "bin/ax-helper",
            "--observer",
            root / "bin/ax-observer",
            "--evidence",
            base / "shutdown",
            "--revision",
            REVISION,
            "--binary-sha256",
            digest(root / "bin/scan-gui"),
        ],
        source,
        env,
        timeout=1200,
    )
    flow = read(base / "flow/report.json")
    matrix = read(base / "shutdown/summary.json")
    if (
        not flow.get("passed")
        or flow.get("forced_cleanup")
        or not matrix.get("passed")
        or len(matrix["runs"]) != 16
    ):
        raise RuntimeError(
            "Native reports did not satisfy the complete qualification contract"
        )
    if list(base.rglob("forced-cleanup.json")):
        raise RuntimeError("Forced cleanup cannot satisfy qualification")
    save(
        base / "accepted.json",
        {
            "passed": True,
            "source": REVISION,
            "gui_sha256": digest(root / "bin/scan-gui"),
            "shutdown_cases": 16,
            "release_acceptance": False,
            "retina_scale_claim": "separate window-specific evidence required",
        },
    )
    check_source(root)


def process_identity(pid):
    result = subprocess.run(
        ["ps", "-ww", "-p", str(pid), "-o", "lstart=,command="],
        capture_output=True,
        text=True,
    )
    return result.stdout.strip() if result.returncode == 0 else None


def manual(root):
    value = read(root / "manual-current.json")
    folder = root / "evidence/manual" / value["label"]
    session = read(folder / "session.json")
    if (folder / "exit.json").exists():
        raise RuntimeError(
            "Manual session has already exited; use a new manual-launch label"
        )
    if process_identity(session["pid"]) != session["identity"]:
        raise RuntimeError(
            "Owned GUI PID/start time/executable identity changed; no input or signal sent"
        )
    if digest(root / "bin/scan-gui") != session["binary_sha256"]:
        raise RuntimeError("Manual binary changed; no input sent")
    return folder, session


def worker(root, label):
    folder = root / "evidence/manual" / label
    spec = importlib.util.spec_from_file_location(
        "qualification", root / "source/scripts/platform-qualification.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    env = module.prepare(folder / "fixture")
    env.pop("FYNE_SCALE", None)
    process = subprocess.Popen(
        [str(root / "bin/scan-gui")],
        env=env,
        stdout=(folder / "gui.stdout.txt").open("w"),
        stderr=(folder / "gui.stderr.txt").open("w"),
    )
    save(
        folder / "session.json",
        {
            "pid": process.pid,
            "identity": process_identity(process.pid),
            "binary_sha256": digest(root / "bin/scan-gui"),
            "source": REVISION,
            "supervisor_pid": os.getpid(),
            "started_utc": stamp(),
        },
    )
    code = process.wait()
    save(
        folder / "exit.json",
        {
            "pid": process.pid,
            "exit_code": code,
            "exited_utc": stamp(),
            "forced_cleanup": (folder / "forced-cleanup.json").exists(),
        },
    )


def manual_launch(root, label):
    native_host()
    check_source(root)
    if (root / "manual-current.json").exists():
        old = read(root / "manual-current.json")
        if not (root / "evidence/manual" / old["label"] / "exit.json").exists():
            raise RuntimeError(
                "Close or clean up the previous owned manual session first"
            )
    label = validate_label(label)
    folder = root / "evidence/manual" / label
    folder.mkdir(parents=True, exist_ok=False)
    save(root / "manual-current.json", {"label": label})
    subprocess.Popen(
        [
            sys.executable,
            str(KIT / "qualify.py"),
            "_manual-worker",
            "--workspace",
            str(root),
            "--label",
            label,
        ],
        env=environment(root),
        start_new_session=True,
        stdin=subprocess.DEVNULL,
        stdout=(folder / "supervisor.stdout.txt").open("w"),
        stderr=(folder / "supervisor.stderr.txt").open("w"),
    )
    deadline = time.monotonic() + 10
    while not (folder / "session.json").exists():
        if time.monotonic() >= deadline:
            raise RuntimeError(
                "Manual launcher did not report its owned process; inspect supervisor logs"
            )
        time.sleep(0.1)
    time.sleep(3)
    print("Owned session:", read(folder / "session.json"))
    print("Scan only:", folder / "fixture/scan-root")


def action(root, label, keys, title):
    folder, session = manual(root)
    target = folder / "actions" / validate_label(label)
    if target.exists():
        raise RuntimeError("Use a new action label; evidence is never overwritten")
    run(
        root,
        "manual-action-" + label,
        [root / "bin/ax-helper", str(session["pid"]), target, keys, title],
        timeout=25,
    )


def capture(root, label, title, control, scale, click):
    folder, session = manual(root)
    target = folder / "captures" / validate_label(label)
    target.mkdir(parents=True, exist_ok=False)
    run(
        root,
        "capture-before-" + label,
        [root / "bin/ax-helper", str(session["pid"]), target / "before", "", title],
        timeout=25,
    )
    run(
        root,
        "display-and-native-hit-" + label,
        [
            root / "bin/display-probe",
            str(session["pid"]),
            target / "display.json",
            title,
            control,
            str(scale),
        ],
        timeout=20,
    )
    record = read(target / "display.json")
    record.update(recorded_utc=stamp(), actual_pointer_click=False)
    if click:
        if control not in ["Add Folder", "Browse root 1"]:
            raise RuntimeError(
                "Physical-click test is limited to the named synthetic picker buttons"
            )
        before = (target / "before/accessibility-after.json").read_text()
        if "Folder path: type a path, then Enter or Go" in before:
            raise RuntimeError(
                "Close the existing picker before testing a fresh pointer action"
            )
        point = record["control"]
        keys = f"click:{point['client_x']:.3f}:{point['client_y']:.3f}"
        run(
            root,
            "actual-pointer-" + label,
            [
                root / "bin/ax-helper",
                str(session["pid"]),
                target / "clicked",
                keys,
                title,
            ],
            timeout=25,
        )
        after = (target / "clicked/accessibility-after.json").read_text()
        if "Folder path: type a path, then Enter or Go" not in after:
            raise RuntimeError(
                "Physical pointer click did not open the expected synthetic folder picker"
            )
        record["actual_pointer_click"] = True
        run(
            root,
            "picker-cancel-" + label,
            [
                root / "bin/ax-helper",
                str(session["pid"]),
                target / "closed",
                "axpress:Cancel",
                title,
            ],
            timeout=25,
        )
    save(target / "accepted.json", record)
    print("Saved window-specific scale, hit-test and screenshots:", target)


def quit_manual(root, title):
    folder, session = manual(root)
    start = time.monotonic()
    run(
        root,
        "manual-native-quit",
        [root / "bin/ax-helper", str(session["pid"]), folder / "quit", "cmd+q", title],
        timeout=8,
    )
    while not (folder / "exit.json").exists():
        if time.monotonic() - start >= 8:
            raise RuntimeError(
                "Manual natural exit was not confirmed within the total eight-second budget"
            )
        time.sleep(0.02)
    report = read(folder / "exit.json")
    if (
        report["exit_code"] != 0
        or report["forced_cleanup"]
        or time.monotonic() - start >= 8
    ):
        raise RuntimeError("Manual exit did not meet natural exit-zero deadline")
    report["total_quit_ms"] = (time.monotonic() - start) * 1000
    save(folder / "accepted-quit.json", report)


def cleanup(root):
    folder, session = manual(root)
    save(
        folder / "forced-cleanup.json",
        {
            "pid": session["pid"],
            "reason": "explicit owned failure cleanup",
            "time": stamp(),
        },
    )
    os.kill(session["pid"], signal.SIGQUIT)
    deadline = time.monotonic() + 3
    while (
        process_identity(session["pid"]) == session["identity"]
        and time.monotonic() < deadline
    ):
        time.sleep(0.05)
    if process_identity(session["pid"]) == session["identity"]:
        os.kill(session["pid"], signal.SIGKILL)
    print("Owned failed session cleaned; this is not a passing Quit result")


def record(root, case, status, notes, evidence):
    if not notes.strip():
        raise RuntimeError(
            "Record a concrete observation or reason, not an empty status"
        )
    path = root / "evidence/operator-report.json"
    report = read(path) if path.exists() else {"cases": {}, "release_acceptance": False}
    for relative in evidence:
        file = (root / "evidence" / relative).resolve()
        if not file.is_relative_to(root / "evidence") or not file.exists():
            raise RuntimeError(
                "Evidence reference must exist inside this run's evidence directory: "
                + relative
            )
    case = validate_label(case)
    if case in report["cases"]:
        raise RuntimeError(
            "Case already recorded; use a new case ID and retain the original result"
        )
    report["cases"][case] = {
        "status": status,
        "notes": notes,
        "evidence": evidence,
        "recorded_utc": stamp(),
    }
    save(path, report)


def package(root):
    if (root / "manual-current.json").exists():
        label = read(root / "manual-current.json")["label"]
        folder = root / "evidence/manual" / label
        if not (folder / "exit.json").exists():
            raise RuntimeError(
                "An owned manual session is not yet closed; use manual-quit or cleanup first"
            )
    source_error = None
    try:
        check_source(root)
    except Exception as error:
        source_error = str(error)
    captures = []
    for path in sorted((root / "evidence/manual").glob("*/captures/*/accepted.json")):
        value = read(path)
        captures.append(
            {
                "evidence": str(path.relative_to(root / "evidence")),
                "pid": value["pid"],
                "title": value["title"],
                "scale": value["actual_backing_scale"],
                "pointer_click": value["actual_pointer_click"],
                "recorded_utc": value["recorded_utc"],
            }
        )
    summary = {
        "source": REVISION,
        "origin": ORIGIN,
        "packaged_utc": stamp(),
        "host": read(root / "evidence/host.json")
        if (root / "evidence/host.json").exists()
        else {"initialization_incomplete": True},
        "operator": read(root / "evidence/operator-report.json")
        if (root / "evidence/operator-report.json").exists()
        else {"cases": {}},
        "source_validation_error": source_error,
        "build": read(root / "evidence/build.json")
        if (root / "evidence/build.json").exists()
        else None,
        "checks": read(root / "evidence/checks-summary.json")
        if (root / "evidence/checks-summary.json").exists()
        else None,
        "automated_runs": [
            str(p.relative_to(root / "evidence"))
            for p in (root / "evidence/automated").glob("*/accepted.json")
        ],
        "display_samples": captures,
        "review_status": "Evidence returned for owner review; no automatic platform acceptance",
        "formal_platform_qualification_complete": False,
        "spoken_reader_qualification": False,
        "runtime_convergence": False,
        "production_pin_adoption": False,
        "release_publication": False,
        "release_acceptance": False,
    }
    save(root / "evidence/SUMMARY.json", summary)
    items, excluded, screenshots = [], [], []
    for path in sorted((root / "evidence").rglob("*")):
        relative = path.relative_to(root / "evidence")
        mode = path.lstat().st_mode
        if stat.S_ISDIR(mode):
            continue
        if (
            not stat.S_ISREG(mode)
            or any(p in [".git", ".ssh"] for p in relative.parts)
            or path.suffix == ".bundle"
        ):
            excluded.append(str(relative))
            continue
        items.append(
            (path, "scanner-macos-arm64-results/evidence/" + relative.as_posix())
        )
        if path.suffix.lower() == ".png":
            data = path.read_bytes()[:24]
            if len(data) < 24 or data[:8] != b"\x89PNG\r\n\x1a\n":
                raise RuntimeError("Invalid screenshot file: " + str(relative))
            width, height = struct.unpack(">II", data[16:24])
            screenshots.append(
                {
                    "path": str(relative),
                    "width": width,
                    "height": height,
                    "sha256": digest(path),
                }
            )
    save(root / "evidence/screenshot-index.json", screenshots)
    save(root / "evidence/package-exclusions.json", excluded)
    items += [
        (
            root / "evidence/screenshot-index.json",
            "scanner-macos-arm64-results/evidence/screenshot-index.json",
        ),
        (
            root / "evidence/package-exclusions.json",
            "scanner-macos-arm64-results/evidence/package-exclusions.json",
        ),
    ]
    items += [
        (p, "scanner-macos-arm64-results/handoff-kit/" + p.name)
        for p in KIT.iterdir()
        if p.is_file() and p.suffix in [".py", ".swift"]
    ]
    unique = {name: path for path, name in items}
    manifest = {name: digest(path) for name, path in sorted(unique.items())}
    name = (
        "scanner-macos-arm64-results-"
        + dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
        + "-"
        + uuid.uuid4().hex[:6]
        + ".zip"
    )
    archive = root / "share" / name
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED, compresslevel=6) as z:
        for name, path in sorted(unique.items()):
            z.write(path, name)
        z.writestr(
            "scanner-macos-arm64-results/FILES-SHA256.json",
            json.dumps(manifest, indent=2) + "\n",
        )
    with zipfile.ZipFile(archive) as z:
        if z.testzip() is not None:
            raise RuntimeError("Compressed evidence integrity check failed")
    checksum = archive.with_suffix(archive.suffix + ".sha256")
    checksum.write_text(digest(archive) + "  " + archive.name + "\n")
    print("SHARE PRIVATELY:", archive)
    print("CHECKSUM:", checksum)
    print(
        "Screenshots:",
        len(screenshots),
        "Excluded symlinks/special files:",
        len(excluded),
    )
    print(
        "Review SUMMARY.json, operator-report.json, logs and screenshots before sending"
    )


def main():
    if sys.version_info < (3, 12):
        raise RuntimeError("Run this kit using native Python 3.12 or newer")
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "command",
        choices=[
            "init",
            "setup",
            "build",
            "permissions",
            "checks",
            "automated",
            "manual-launch",
            "action",
            "capture",
            "manual-quit",
            "cleanup",
            "record",
            "package",
            "_manual-worker",
        ],
    )
    parser.add_argument("--workspace", type=Path, required=True)
    parser.add_argument("--label", default="retina")
    parser.add_argument("--request", action="store_true")
    parser.add_argument("--keys", default="")
    parser.add_argument("--title", choices=["Scanner", "Settings"], default="Scanner")
    parser.add_argument("--control", default="Add Folder")
    parser.add_argument("--expected-scale", type=float, choices=[1, 2], default=2)
    parser.add_argument("--click", action="store_true")
    parser.add_argument("--case", default="")
    parser.add_argument(
        "--status",
        choices=["passed", "failed", "pending", "not_run"],
        default="pending",
    )
    parser.add_argument("--notes", default="")
    parser.add_argument("--evidence", action="append", default=[])
    args = parser.parse_args()
    if args.command == "init":
        initialize(args.workspace)
        return
    root = load(args.workspace)
    commands = {
        "setup": setup,
        "build": build,
        "checks": checks,
        "package": package,
        "cleanup": cleanup,
    }
    if args.command in commands:
        commands[args.command](root)
    elif args.command == "permissions":
        permissions(root, args.request)
    elif args.command == "automated":
        automated(root, args.label)
    elif args.command == "manual-launch":
        manual_launch(root, args.label)
    elif args.command == "_manual-worker":
        worker(root, args.label)
    elif args.command == "action":
        action(root, args.label, args.keys, args.title)
    elif args.command == "capture":
        capture(
            root, args.label, args.title, args.control, args.expected_scale, args.click
        )
    elif args.command == "manual-quit":
        quit_manual(root, args.title)
    elif args.command == "record":
        record(root, args.case, args.status, args.notes, args.evidence)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print("STOP:", error, file=sys.stderr)
        print(
            "Retain the failed logs/fixture. Do not change source or relax timeouts to manufacture a pass.",
            file=sys.stderr,
        )
        sys.exit(1)
