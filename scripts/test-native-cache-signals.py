#!/usr/bin/env python3
"""Require the real Linux private-bus cache-signal regression; zero tests fail."""
from pathlib import Path
import re
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
if sys.platform != "linux":
    raise SystemExit("This gate requires Linux and a private D-Bus session")
result = subprocess.run([
    "dbus-run-session", "--", "cargo", "+1.88", "test", "--locked",
    "--manifest-path", str(root / "tools/project-catalog/native-a11y/Cargo.toml"),
    "-p", "accesskit_unix", "cache_signals_preserve_struct_arguments", "--", "--ignored",
], cwd=root, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, timeout=120)
print(result.stdout, end="")
if result.returncode or not re.search(r"test result: ok\. 1 passed; 0 failed; 0 ignored;", result.stdout):
    raise SystemExit("Required cache-signal test did not execute successfully")
