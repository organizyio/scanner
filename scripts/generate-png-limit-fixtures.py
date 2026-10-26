#!/usr/bin/env python3
"""Header-only negative controls: valid CRCs, deliberately absent pixel payload."""

from pathlib import Path
import hashlib
import json
import struct
import zlib

ROOT = Path(__file__).resolve().parents[1] / "crates/phash/tests/limits"


def chunk(kind, data):
    return (
        struct.pack(">I", len(data))
        + kind
        + data
        + struct.pack(">I", zlib.crc32(kind + data) & 0xFFFFFFFF)
    )


def main():
    ROOT.mkdir(parents=True, exist_ok=True)
    manifest = {}
    for name, width, height in [
        ("width.png", 16385, 1),
        ("height.png", 1, 16385),
        ("decoded-budget.png", 4096, 4097),
    ]:
        data = (
            b"\x89PNG\r\n\x1a\n"
            + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0))
            + chunk(b"IDAT", b"")
            + chunk(b"IEND", b"")
        )
        (ROOT / name).write_bytes(data)
        manifest[name] = {
            "sha256": hashlib.sha256(data).hexdigest(),
            "width": width,
            "height": height,
            "expected": "resource limit before decoding missing pixel payload",
        }
    (ROOT / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")


if __name__ == "__main__":
    main()
