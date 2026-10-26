"""Deterministic, bounded, synthetic corpus recipes; no external tools/data."""

import hashlib
import io
import json
import struct
import zipfile
import zlib

CLASS_NAMES = ("empty", "medium", "large", "huge", "image", "document", "pdf", "tiny")


def image():
    def chunk(kind, body):
        return (
            struct.pack(">I", len(body))
            + kind
            + body
            + struct.pack(">I", zlib.crc32(kind + body) & 0xFFFFFFFF)
        )

    width, height = 1024, 768
    data = b"".join(
        b"\0"
        + bytes(v for x in range(width) for v in ((x + y) % 256, x % 256, y % 256))
        for y in range(height)
    )
    return (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
        + chunk(b"IDAT", zlib.compress(data, 6))
        + chunk(b"IEND", b"")
    )


def document():
    xml = (
        "<cp:coreProperties xmlns:cp='urn:cp' xmlns:dc='urn:dc'><dc:creator>Scanner Synthetic</dc:creator><cp:lastModifiedBy>Fixture generator</cp:lastModifiedBy><cp:revision>007</cp:revision><dc:description>"
        + "x" * (512 << 10)
        + "</dc:description></cp:coreProperties>"
    ).encode()
    data = io.BytesIO()
    with zipfile.ZipFile(data, "w", zipfile.ZIP_DEFLATED, compresslevel=6) as z:
        files = {
            "docProps/core.xml": xml,
            "[Content_Types].xml": b'<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>',
            "_rels/.rels": b'<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>',
            "word/document.xml": b'<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body/></w:document>',
        }
        for name, body in files.items():
            info = zipfile.ZipInfo(name, date_time=(2020, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100600 << 16
            z.writestr(info, body, compresslevel=6)
    return data.getvalue()


def pdf():
    pages = 32
    objects = [
        b"<< /Type /Catalog /Pages 2 0 R >>",
        (
            "<< /Type /Pages /Count 32 /Kids ["
            + " ".join(f"{i + 4} 0 R" for i in range(pages))
            + "] >>"
        ).encode(),
        b"<< /Length 0 >>\nstream\n\nendstream",
    ]
    objects += [
        b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Contents 3 0 R >>"
    ] * pages
    data, offsets = bytearray(b"%PDF-1.4\n"), [0]
    for i, body in enumerate(objects, 1):
        offsets.append(len(data))
        data.extend(f"{i} 0 obj\n".encode() + body + b"\nendobj\n")
    start = len(data)
    data.extend(f"xref\n0 {len(offsets)}\n0000000000 65535 f \n".encode())
    data.extend(b"".join(f"{pos:010} 00000 n \n".encode() for pos in offsets[1:]))
    data.extend(
        f"trailer\n<< /Size {len(offsets)} /Root 1 0 R >>\nstartxref\n{start}\n%%EOF\n".encode()
    )
    return bytes(data)


def recipes(profile):
    if profile == "small":
        return [("tiny", "txt", b"x" * 256)]
    if profile != "mixed":
        raise ValueError("Unknown scale profile")
    block = bytes(range(256)) * 256
    return [
        ("empty", "txt", b""),
        ("medium", "bin", block),
        ("large", "bin", block * 16),
        ("huge", "bin", block * 128),
        ("image", "png", image()),
        ("document", "docx", document()),
        ("pdf", "pdf", pdf()),
        ("tiny", "txt", b"x" * 256),
    ]


def class_index(i, profile):
    return min(i % 128, 7) if profile == "mixed" else 0


def specification(files, profile, bodies=None):
    bodies = recipes(profile) if bodies is None else bodies
    classes = [
        {
            "name": name,
            "extension": ext,
            "size": len(data),
            "sha256": hashlib.sha256(data).hexdigest(),
        }
        for name, ext, data in bodies
    ]
    counts = {v["name"]: 0 for v in classes}
    for i in range(files):
        counts[classes[class_index(i, profile)]["name"]] += 1
    return {
        "schema": 1,
        "profile": profile,
        "classes": classes,
        "class_counts": counts,
        "total_bytes": sum(c["size"] * counts[c["name"]] for c in classes),
        "image_dimensions": [1024, 768] if profile == "mixed" else None,
        "docx_uncompressed_description_bytes": 512 << 10
        if profile == "mixed"
        else None,
        "pdf_pages": 32 if profile == "mixed" else None,
    }


def create(root, files, profile):
    bodies = recipes(profile)
    spec = specification(files, profile, bodies)
    for i in range(files):
        name, ext, data = bodies[class_index(i, profile)]
        folder = root / ("alpha" if i % 2 == 0 else "unicode-β") / f"shard-{i % 32:02}"
        folder.mkdir(parents=True, exist_ok=True)
        (folder / f"file-{i}.{ext}").write_bytes(data)
    (root / "fixture-spec.json").write_text(json.dumps(spec, indent=2) + "\n")
    return spec
