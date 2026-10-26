#!/usr/bin/env python3
"""Synthetic PNG images with an independent gradient oracle; Python stdlib only."""

import hashlib
import json
from pathlib import Path
import struct
import zlib

ROOT = Path(__file__).resolve().parents[1] / "crates/phash/tests/fixtures"


def chunk(kind, body):
    return (
        struct.pack(">I", len(body))
        + kind
        + body
        + struct.pack(">I", zlib.crc32(kind + body) & 0xFFFFFFFF)
    )


def png(color, depth=8, interlaced=False, constant=False):
    channels = {0: 1, 2: 3, 3: 1, 4: 2, 6: 4}[color]

    def pixel(x, y):
        v = 128 if constant else (x * 28 if y % 2 == 0 else (8 - x) * 28)
        if color == 3:
            return bytes([v])
        values = ([v] if color in (0, 4) else [v] * 3) + (
            [255] if color in (4, 6) else []
        )
        assert len(values) == channels
        return b"".join(
            struct.pack(">H", v * 257) if depth == 16 else bytes([v]) for v in values
        )

    passes = (
        [(0, 0, 1, 1)]
        if not interlaced
        else [
            (0, 0, 8, 8),
            (4, 0, 8, 8),
            (0, 4, 4, 8),
            (2, 0, 4, 4),
            (0, 2, 2, 4),
            (1, 0, 2, 2),
            (0, 1, 1, 2),
        ]
    )
    raw = b"".join(
        b"\0" + b"".join(pixel(x, y) for x in range(x0, 9, dx))
        for x0, y0, dx, dy in passes
        for y in range(y0, 8, dy)
    )
    palette = (
        chunk(b"PLTE", bytes(v for n in range(256) for v in (n, n, n)))
        + chunk(b"tRNS", bytes([255] * 256))
        if color == 3
        else b""
    )
    return (
        b"\x89PNG\r\n\x1a\n"
        + chunk(
            b"IHDR", struct.pack(">IIBBBBB", 9, 8, depth, color, 0, 0, int(interlaced))
        )
        + palette
        + chunk(b"IDAT", zlib.compress(raw, 6))
        + chunk(b"IEND", b"")
    )


def main():
    ROOT.mkdir(parents=True, exist_ok=True)
    cases = {
        "rgb.png": png(2),
        "rgba.png": png(6),
        "gray.png": png(0),
        "gray-alpha.png": png(4),
        "indexed.png": png(3),
        "gray16.png": png(0, 16),
        "rgb16.png": png(2, 16),
        "adam7.png": png(2, interlaced=True),
        "constant.png": png(2, constant=True),
    }
    manifest = {}
    for name, body in cases.items():
        (ROOT / name).write_bytes(body)
        # No resizing: gradient samples 9x8. Alternate ascending/descending rows
        # yield eight true/false comparisons, or all false for a constant image.
        manifest[name] = {
            "sha256": hashlib.sha256(body).hexdigest(),
            "phash_u64": 0 if name == "constant.png" else 0xFF00FF00FF00FF00,
        }
    (ROOT / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")


if __name__ == "__main__":
    main()
