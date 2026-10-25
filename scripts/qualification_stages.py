"""Cross-process deadlines for an owned, fresh native qualification fixture.

The supervisor and worker share the host monotonic clock. Completion retains
timestamps so a late worker cannot erase a missed deadline between polls.
"""

import json
import os
import time
from pathlib import Path


def write_stage(root, value):
    target = Path(root) / "native-stage.json"
    temporary = target.with_suffix(".tmp")
    temporary.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")
    os.replace(temporary, target)


def begin(root, nonce, name, limit_seconds=8):
    if not 0 < limit_seconds <= 8:
        raise ValueError("native stage limit must be between zero and eight seconds")
    value = {
        "nonce": nonce,
        "name": name,
        "pid": os.getpid(),
        "start_ns": time.monotonic_ns(),
        "limit_ns": int(limit_seconds * 1e9),
    }
    write_stage(root, value)
    return value


def finish(root, value):
    value = dict(value, finish_ns=time.monotonic_ns())
    write_stage(root, value)
    if expired(root, value["nonce"]):
        raise RuntimeError(
            "native stage completed after its deadline: " + value["name"]
        )
    return (value["finish_ns"] - value["start_ns"]) / 1e6


def expired(root, nonce, now_ns=None):
    target = Path(root) / "native-stage.json"
    if not target.exists():
        return False
    value = json.loads(target.read_text(encoding="utf-8"))
    if value["nonce"] != nonce or not 0 < value["limit_ns"] <= 8_000_000_000:
        raise ValueError("native stage does not match owned run or bounded limit")
    now = time.monotonic_ns() if now_ns is None else now_ns
    end = value.get("finish_ns", now)
    if not isinstance(end, int) or not 0 <= value["start_ns"] <= end:
        raise ValueError("invalid monotonic stage timestamps")
    return end - value["start_ns"] >= value["limit_ns"]
