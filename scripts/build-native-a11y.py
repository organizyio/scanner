#!/usr/bin/env python3
"""Build the pinned, isolated native GUI accessibility adapter from source."""
import argparse
import os
from pathlib import Path
import platform
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[1]
MODULE = ROOT / "tools/project-catalog/native-a11y"


def build(check=False):
    env = dict(os.environ)
    # The cgo wrapper has a stable host-local link location; isolate this Cargo
    # workspace from inherited preferred-Scanner build-cache overrides.
    env.pop("CARGO_TARGET_DIR", None)
    args = ["cargo", "+1.88"]
    target = []
    if platform.system() == "Windows":
        arch = platform.machine().lower()
        if arch not in ("amd64", "x86_64"):
            raise RuntimeError("Native Windows GUI qualification currently requires x64")
        triple = "x86_64-pc-windows-gnu"
        subprocess.run(["rustup", "target", "add", "--toolchain", "1.88", triple], check=True)
        target = ["--target", triple]
    common = ["--manifest-path", str(MODULE / "Cargo.toml"), "--locked"]
    if check:
        subprocess.run(args + ["fmt", "--manifest-path", str(MODULE / "Cargo.toml"), "--", "--check"], check=True, env=env)
        subprocess.run(args + ["clippy"] + common + target + ["--all-targets", "--", "-D", "warnings"], check=True, env=env)
        subprocess.run(args + ["test"] + common + target, check=True, env=env)
        subprocess.run(args + ["doc"] + common + target + ["--no-deps"], check=True, env=env)
    subprocess.run(args + ["build"] + common + target + ["--release"], check=True, env=env)
    if target:
        artifact = MODULE / "target" / target[1] / "release/libscanner_native_a11y.a"
        destination = MODULE / "target/release/libscanner_native_a11y.a"
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(artifact, destination)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="also run pinned formatting, clippy and tests")
    build(parser.parse_args().check)
