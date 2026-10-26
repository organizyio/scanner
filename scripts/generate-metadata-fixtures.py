#!/usr/bin/env python3
"""Synthetic preferred metadata controls; no external source documents."""

from pathlib import Path
import hashlib
import io
import json
import struct
import zipfile

ROOT = Path(__file__).resolve().parents[1] / "crates/metadata/tests/fixtures"


def docx(oversize=False):
    xml = "<cp:coreProperties xmlns:cp='urn:cp' xmlns:dc='urn:dc'><dc:creator>Synthetic &amp; é🙂</dc:creator><cp:revision>007</cp:revision><dc:description>"
    xml += "x" * ((2 << 20) if oversize else 32)
    xml += "</dc:description></cp:coreProperties>"
    data = io.BytesIO()
    with zipfile.ZipFile(data, "w") as z:
        info = zipfile.ZipInfo("docProps/core.xml", (2020, 1, 1, 0, 0, 0))
        info.create_system = 3
        info.external_attr = 0o100600 << 16
        info.compress_type = zipfile.ZIP_DEFLATED
        z.writestr(info, xml.encode(), compresslevel=6)
    return data.getvalue()


def pdf(cycle=False):
    objects = [
        b"<< /Type /Catalog /Pages 2 0 R >>",
        b"<< /Type /Pages /Kids [3 0 R"
        + (b" 2 0 R" if cycle else b"")
        + b"] /Count 1 >>",
        b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] >>",
    ]
    data = bytearray(b"%PDF-1.5\n")
    offsets = []
    for i, body in enumerate(objects, 1):
        offsets.append(len(data))
        data.extend(f"{i} 0 obj\n".encode() + body + b"\nendobj\n")
    xref = len(data)
    data.extend(b"xref\n0 4\n0000000000 65535 f \n")
    data.extend(b"".join(f"{n:010} 00000 n \n".encode() for n in offsets))
    data.extend(
        f"trailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n{xref}\n%%EOF\n".encode()
    )
    return bytes(data)


def tiff(oversize=False):
    fields = [
        (0x010F, 2, b"Synthetic\0"),
        (50000, 4, struct.pack("<I", 0xFFFFFFFF)),
        (50001, 1, bytes([0, 255])),
    ]
    if oversize:
        fields = [(50002, 7, b"x" * (262144 + 1))]
    header = bytearray(b"II*\0" + struct.pack("<I", 8) + struct.pack("<H", len(fields)))
    payload = bytearray()
    for tag, kind, body in fields:
        length = len(body) // (4 if kind == 4 else 1)
        value = (
            body.ljust(4, b"\0")
            if len(body) <= 4
            else struct.pack("<I", 8 + 2 + 12 * len(fields) + 4 + len(payload))
        )
        header.extend(struct.pack("<HHI", tag, kind, length) + value)
        if len(body) > 4:
            payload.extend(body)
    return bytes(header) + struct.pack("<I", 0) + payload


def main():
    ROOT.mkdir(parents=True, exist_ok=True)
    files = {
        "valid.docx": docx(),
        "core-budget.docx": docx(True),
        "valid.pdf": pdf(),
        "cycle.pdf": pdf(True),
        "valid.tiff": tiff(),
        "values-budget.tiff": tiff(True),
    }
    manifest = {}
    for name, body in files.items():
        (ROOT / name).write_bytes(body)
        manifest[name] = {
            "sha256": hashlib.sha256(body).hexdigest(),
            "bytes": len(body),
            "metadata_expected": name.startswith("valid."),
        }
    (ROOT / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")


if __name__ == "__main__":
    main()
