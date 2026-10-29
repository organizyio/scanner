#!/usr/bin/env python3
"""Inspect a returned Mac ZIP without extracting/executing it or approving a gate."""

import argparse
import collections
import datetime as dt
import hashlib
import json
import math
from pathlib import Path, PurePosixPath
import re
import stat
import struct
import unicodedata
import zipfile
import zlib

ROOT = "scanner-macos-arm64-results/"
REVISION = "c16955e76e4768c74d77fa3b4b3ac1a80c0af58d"
KIT = {
    "qualify.py": "6e2a1f0a17b08c98a3f4d4cc847a06af2e28116be4f256904f0fa906cc303023",
    "display-probe.swift": "85cabef5c186e9f6807c653863a2bb10ba6a0d54651a5bb021891132a8b987da",
}
CHECKS = {
    "preferred-rust-checks",
    "qualification-regressions",
    "catalog-vet",
    "catalog-tests",
    "gui-native-race",
    "fyne-queue-race",
    "native-owner-lifecycle-build",
    "native-owner-lifecycle-run",
    "compat-engine-fmt",
    "compat-engine-clippy",
    "compat-engine-tests",
    "build-local-engine",
    "build-embedded-wrapper",
    "wrapper-vet",
    "wrapper-unit",
    "wrapper-real-engine-integration",
    "public-history-source-contracts",
    "filesystem",
    "signals",
    "cli-native-permission-cases",
}
MAX_FILES, MAX_MEMBER, MAX_TOTAL, MAX_JSON = 10000, 64 << 20, 512 << 20, 4 << 20
SHA = re.compile(r"[0-9a-f]{64}\Z")
FLAGS = (
    "formal_platform_qualification_complete",
    "spoken_reader_qualification",
    "runtime_convergence",
    "production_pin_adoption",
    "release_publication",
    "release_acceptance",
)


class Invalid(ValueError):
    pass


def require(value, message):
    if not value:
        raise Invalid(message)


def pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, "Duplicate JSON key")
        result[key] = value
    return result


def finite_float(value):
    number = float(value)
    require(math.isfinite(number), "Nonfinite JSON number")
    return number


def load_json(data):
    require(len(data) <= MAX_JSON, "JSON member exceeds limit")
    return json.loads(
        data,
        object_pairs_hook=pairs,
        parse_float=finite_float,
        parse_constant=lambda _: (_ for _ in ()).throw(
            Invalid("Nonfinite JSON number")
        ),
    )


def safe_name(name):
    require(
        isinstance(name, str)
        and name
        and len(name.encode("utf-8")) <= 1024
        and len(name.split("/")) <= 32
        and unicodedata.normalize("NFC", name) == name,
        "Noncanonical archive path",
    )
    require(
        not any(ord(c) < 32 or ord(c) == 127 for c in name), "Control character in path"
    )
    require(
        "\\" not in name and ":" not in name and not name.startswith("/"),
        "Absolute/ambiguous path",
    )
    value = name.removesuffix("/")
    require(
        all(
            p and p not in (".", "..") and not p.endswith((" ", "."))
            for p in value.split("/")
        ),
        "Traversal/ambiguous path",
    )
    require(
        value == ROOT.rstrip("/") or value.startswith(ROOT), "Unexpected archive root"
    )
    parts = PurePosixPath(value).parts
    reserved = {"con", "prn", "aux", "nul"} | {
        f"{p}{i}" for p in ("com", "lpt") for i in range(1, 10)
    }
    require(
        not any(p.split(".")[0].casefold() in reserved for p in parts),
        "Reserved device name",
    )
    require(
        not any(p.casefold() in (".git", ".ssh", ".env") for p in parts),
        "Private storage in package",
    )
    require(
        not value.lower().endswith(
            (".bundle", ".pem", ".key", ".exe", ".dll", ".dylib", ".a")
        ),
        "Recovery, credential or executable artifact in package",
    )
    if value != ROOT.rstrip("/"):
        require(
            parts[1] in ("evidence", "handoff-kit", "FILES-SHA256.json"),
            "Unexpected package content",
        )
    return value


def png_geometry(data):
    require(data[:8] == b"\x89PNG\r\n\x1a\n", "Invalid PNG signature")
    offset, geometry, image, ended = 8, None, False, False
    while offset < len(data):
        require(offset + 12 <= len(data), "Truncated PNG")
        size = struct.unpack(">I", data[offset : offset + 4])[0]
        kind = data[offset + 4 : offset + 8]
        require(offset + 12 + size <= len(data), "Truncated PNG chunk")
        body = data[offset + 8 : offset + 8 + size]
        crc = struct.unpack(">I", data[offset + 8 + size : offset + 12 + size])[0]
        require(zlib.crc32(kind + body) & 0xFFFFFFFF == crc, "PNG CRC mismatch")
        if geometry is None:
            require(kind == b"IHDR" and size == 13, "PNG has no initial IHDR")
            width, height = struct.unpack(">II", body[:8])
            require(
                0 < width <= 16384
                and 0 < height <= 16384
                and width * height <= 100000000,
                "PNG dimensions exceed limits",
            )
            geometry = (width, height)
        elif kind == b"IHDR":
            raise Invalid("Duplicate PNG header")
        image |= kind == b"IDAT" and size > 0
        offset += 12 + size
        if kind == b"IEND":
            require(size == 0 and offset == len(data), "PNG trailing data")
            ended = True
            break
    require(geometry and image and ended, "Incomplete PNG")
    return geometry


def zip_envelope(path):
    # Bound the central directory before ZipFile allocates its member list.
    size = path.stat().st_size
    require(size <= MAX_TOTAL, "Archive exceeds compressed size limit")
    with path.open("rb") as stream:
        require(stream.read(4) == b"PK\x03\x04", "Not a plain ZIP archive")
        stream.seek(max(0, size - 65557))
        tail = stream.read()
    pos = tail.rfind(b"PK\x05\x06")
    require(pos >= 0 and pos + 22 <= len(tail), "Missing ZIP end record")
    _, disk, start_disk, count, total, directory_size, offset, comment = struct.unpack(
        "<4s4H2LH", tail[pos : pos + 22]
    )
    require(
        disk == start_disk == 0 and count == total and 0 < total <= MAX_FILES,
        "Split/ZIP64 archive or excessive members",
    )
    require(
        directory_size <= 8 << 20
        and offset + directory_size == size - len(tail) + pos
        and pos + 22 + comment == len(tail),
        "Invalid/oversize ZIP directory or trailer",
    )


def inspect_archive(path, kit=None):
    kit = KIT if kit is None else kit
    """Stream/hash bounded members; retain only bounded JSON and PNG geometry."""
    documents, hashes, pictures, names, total = {}, {}, {}, set(), 0
    json_total = 0
    zip_envelope(path)
    with zipfile.ZipFile(path) as archive:
        require(len(archive.infolist()) <= MAX_FILES, "Too many ZIP members")
        for info in archive.infolist():
            require(info.orig_filename == info.filename, "Ambiguous ZIP member name")
            name = safe_name(info.filename)
            canonical = name.casefold()
            require(canonical not in names, "Duplicate/case-colliding ZIP member")
            names.add(canonical)
            mode = info.external_attr >> 16
            require(
                stat.S_IFMT(mode)
                in (0, stat.S_IFDIR if info.is_dir() else stat.S_IFREG),
                "Symlink or special ZIP member",
            )
            require(not info.flag_bits & 1, "Encrypted ZIP member")
            require(
                info.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED),
                "Unsupported compression",
            )
            require(0 <= info.file_size <= MAX_MEMBER, "ZIP member exceeds size limit")
            if info.is_dir():
                require(info.file_size == 0, "Directory with payload")
                continue
            require(
                not (
                    info.file_size > (1 << 20)
                    and info.file_size > max(1, info.compress_size) * 1000
                ),
                "Excessive compression ratio",
            )
            total += info.file_size
            require(total <= MAX_TOTAL, "ZIP expanded size exceeds limit")
            keep = name.endswith((".json", ".png"))
            require(
                not name.endswith(".json") or info.file_size <= MAX_JSON,
                "JSON member exceeds limit",
            )
            digest, chunks, size = hashlib.sha256(), [], 0
            with archive.open(info) as stream:
                for block in iter(lambda: stream.read(1 << 20), b""):
                    size += len(block)
                    require(
                        size <= info.file_size and size <= MAX_MEMBER,
                        "ZIP size mismatch",
                    )
                    digest.update(block)
                    if keep:
                        chunks.append(block)
            require(size == info.file_size, "ZIP size mismatch")
            hashes[name] = digest.hexdigest()
            if name.endswith(".json"):
                json_total += size
                require(json_total <= 32 << 20, "Aggregate JSON exceeds limit")
                documents[name] = load_json(b"".join(chunks))
            if name.endswith(".png"):
                pictures[name] = png_geometry(b"".join(chunks))
    manifest = documents.get(ROOT + "FILES-SHA256.json")
    require(isinstance(manifest, dict), "Missing checksum manifest")
    expected = {n: h for n, h in hashes.items() if n != ROOT + "FILES-SHA256.json"}
    require(
        all(
            isinstance(n, str) and isinstance(h, str) and SHA.fullmatch(h)
            for n, h in manifest.items()
        ),
        "Malformed checksum manifest",
    )
    require(manifest == expected, "Manifest membership/hash mismatch")
    for name, value in kit.items():
        require(
            hashes.get(ROOT + "handoff-kit/" + name) == value,
            "Unreviewed handoff helper",
        )
    require(
        {n for n in hashes if n.startswith(ROOT + "handoff-kit/")}
        == {ROOT + "handoff-kit/" + n for n in kit},
        "Unexpected handoff helper",
    )
    return documents, hashes, pictures


def screenshot_fits(width, height, bounds, scale):
    # macOS screencapture -l includes a window shadow in the shipped kit.
    # Allow bounded equal padding, but never infer actual backing scale from it.
    dx, dy = width - bounds["width"] * scale, height - bounds["height"] * scale
    return -4 <= dx <= 256 * scale and -4 <= dy <= 256 * scale and abs(dx - dy) <= 4


def finite_ms(value):
    return type(value) in (int, float) and math.isfinite(value) and 0 <= value < 8000


def trusted_profile(path):
    # Authority comes from an explicitly supplied LOCAL profile prepared by us,
    # never a profile read from the returned archive.
    require(path.stat().st_size <= 4096, "Trusted profile exceeds limit")
    profile = load_json(path.read_bytes())
    require(isinstance(profile, dict) and set(profile) == {"revision", "kit"}, "Invalid trusted profile")
    require(isinstance(profile["revision"], str) and re.fullmatch("[0-9a-f]{40}", profile["revision"]), "Invalid profile revision")
    kit = profile["kit"]
    require(isinstance(kit, dict) and set(kit) == set(KIT), "Invalid profile helper names")
    require(all(isinstance(v, str) and SHA.fullmatch(v) for v in kit.values()), "Invalid profile helper digest")
    return profile


def verify(path, archive_sha256=None, profile=None):
    revision = REVISION if profile is None else profile["revision"]
    kit = KIT if profile is None else profile["kit"]
    if archive_sha256 is not None:
        require(
            isinstance(archive_sha256, str) and SHA.fullmatch(archive_sha256),
            "Invalid expected archive digest",
        )
        require(
            path.stat().st_size <= MAX_TOTAL, "Archive exceeds compressed size limit"
        )
        digest = hashlib.sha256()
        with path.open("rb") as stream:
            for block in iter(lambda: stream.read(1 << 20), b""):
                digest.update(block)
        require(
            digest.hexdigest() == archive_sha256, "External archive digest mismatch"
        )
    documents, hashes, pictures = inspect_archive(path, kit)
    prefix = ROOT + "evidence/"

    def doc(name):
        value = documents.get(prefix + name, {})
        return value if isinstance(value, dict) else {}

    coverage = {}

    def check(name, condition):
        coverage[name] = "consistent" if condition else "missing_or_inconsistent"

    summary, host, build = doc("SUMMARY.json"), doc("host.json"), doc("build.json")
    check(
        "reviewed_source",
        summary.get("source") == revision
        and build.get("source") == revision
        and summary.get("origin") == "https://github.com/organizyio/scanner.git"
        and summary.get("source_validation_error", "missing") is None
        and doc("source-status.json").get("revision") == revision
        and doc("source-status.json").get("status") == "",
    )
    check(
        "native_arm64_host",
        host.get("machine") == "arm64"
        and host.get("python_machine") == "arm64"
        and host.get("root_user") is False,
    )
    check(
        "summary_matches_canonical_reports",
        all(
            summary.get(k) == doc(file)
            for k, file in [
                ("host", "host.json"),
                ("build", "build.json"),
                ("checks", "checks-summary.json"),
                ("operator", "operator-report.json"),
            ]
        ),
    )
    check("acceptance_boundaries", all(summary.get(k) is False for k in FLAGS))
    binaries = build.get("binaries", {})
    binaries = binaries if isinstance(binaries, dict) else {}
    check(
        "binary_identity",
        set(binaries)
        == {"scan-gui", "scan-cli", "ax-helper", "ax-observer", "display-probe"}
        and all(
            isinstance(v, dict)
            and v.get("architecture") == "arm64"
            and isinstance(v.get("sha256"), str)
            and SHA.fullmatch(v["sha256"])
            for v in binaries.values()
        ),
    )
    gui = (
        binaries.get("scan-gui", {}).get("sha256")
        if isinstance(binaries.get("scan-gui", {}), dict)
        else None
    )
    checks = doc("checks-summary.json")
    values = checks.get("checks", {})
    check(
        "build_and_cli_checks",
        checks.get("passed") is True
        and isinstance(values, dict)
        and set(values) == CHECKS
        and all(
            isinstance(v, dict)
            and v.get("passed") is True
            and type(v.get("exit_code")) is int
            and v["exit_code"] == 0
            for v in values.values()
        ),
    )
    journal = documents.get(prefix + "commands.json", [])
    logged = (
        {
            v.get("name")
            for v in journal
            if isinstance(v, dict)
            and type(v.get("exit_code")) is int
            and v["exit_code"] == 0
            and isinstance(v.get("log"), str)
            and prefix + v["log"] in hashes
        }
        if isinstance(journal, list)
        else set()
    )
    check("check_logs_present", CHECKS <= logged)
    index = documents.get(prefix + "screenshot-index.json", [])
    indexed = {}
    if isinstance(index, list):
        for item in index:
            if isinstance(item, dict) and isinstance(item.get("path"), str):
                name = prefix + item["path"]
                if name in indexed:
                    indexed = {}
                    break
                indexed[name] = item
    check(
        "screenshot_inventory",
        bool(pictures)
        and set(indexed) == set(pictures)
        and all(
            (i.get("width"), i.get("height")) == pictures[n]
            and i.get("sha256") == hashes[n]
            for n, i in indexed.items()
        ),
    )
    accepted = sorted(
        n
        for n in documents
        if n.startswith(prefix + "automated/")
        and len(PurePosixPath(n).parts) == 5
        and n.endswith("/accepted.json")
    )
    expected_runs = collections.Counter(
        {
            (m, s): (3 if s == "idle" else 1)
            for m in ("attached", "detach-before-quit", "none")
            for s in ("idle", "settings", "modal")
        }
    )
    expected_runs[("attached", "active")] = 1
    good = []
    for name in accepted:
        base = name.removeprefix(prefix).removesuffix("accepted.json")
        if profile is not None:
            latency = doc(base + "ax-latency/report.json")
            samples = latency.get("samples", [])
            check("serial_ax_latency/" + PurePosixPath(base).name,
                  latency.get("passed") is True and latency.get("source") == revision
                  and latency.get("binary_sha256") == build.get("binaries", {}).get("scan-gui", {}).get("sha256")
                  and latency.get("forced_cleanup") is False
                  and isinstance(samples, list) and bool(samples)
                  and all(isinstance(x, dict) and type(x.get("median_ms")) in (int, float)
                          and 0 <= x["median_ms"] < 10
                          and type(x.get("snapshot_ms")) in (int, float) and 0 <= x["snapshot_ms"] < 5000
                          and x.get("attribute_calls", 0) >= 20 for x in samples)
                  and any(isinstance(x, dict) and x.get("window_count", 0) >= 2 for x in samples))
        a, f, m = (
            doc(base + "accepted.json"),
            doc(base + "flow/report.json"),
            doc(base + "shutdown/summary.json"),
        )
        runs = m.get("runs", [])
        ok = (
            isinstance(gui, str)
            and SHA.fullmatch(gui)
            and a.get("passed") is True
            and a.get("source") == revision
            and a.get("gui_sha256") == gui
            and f.get("source") == revision
            and f.get("binary_sha256") == gui
            and f.get("passed") is True
            and finite_ms(f.get("quit_ms"))
            and f.get("forced_cleanup") is False
            and type(f.get("native_quit_exit")) is int
            and f["native_quit_exit"] == 0
            and all(
                f.get(k) is True
                for k in (
                    "unicode",
                    "folder_error_recovery",
                    "profile_switch_restore",
                    "tree_keyboard",
                    "pause_resume_cancel",
                    "settings_restart_checkpoint",
                )
            )
            and m.get("passed") is True
            and m.get("source") == revision
            and m.get("binary_sha256") == gui
            and isinstance(runs, list)
            and len(runs) == 16
            and all(isinstance(r, dict) for r in runs)
        )
        if ok:
            ok = collections.Counter(
                (r.get("observer_mode"), r.get("scenario")) for r in runs
            ) == expected_runs and all(
                r.get("passed") is True
                and r.get("forced") is False
                and type(r.get("exit_code")) is int
                and r["exit_code"] == 0
                and finite_ms(r.get("quit_ms"))
                and (
                    r.get("observer_detach_ms") is None
                    or finite_ms(r["observer_detach_ms"])
                )
                for r in runs
            )
        check("automated/" + PurePosixPath(base).name, ok)
        if ok:
            good.append(base + "accepted.json")
    reported_runs = summary.get("automated_runs")
    check(
        "automated_run_inventory",
        bool(accepted)
        and isinstance(reported_runs, list)
        and all(isinstance(n, str) for n in reported_runs)
        and len(reported_runs) == len(set(reported_runs))
        and set(reported_runs) == {n.removeprefix(prefix) for n in accepted},
    )
    # Actual display/pointer claims need matching JSON, indexed PNG images and same-process transitions.
    captures = []
    for n in sorted(documents):
        if (
            n.startswith(prefix + "manual/")
            and "/captures/" in n
            and n.endswith("/accepted.json")
        ):
            record = documents[n]
            if not isinstance(record, dict):
                continue
            parent = n.removesuffix("accepted.json")
            window_id = record.get("window_id")
            imgs = [
                p for p in pictures if p == parent + f"before/window-{window_id}.png"
            ]
            try:
                stamp = dt.datetime.fromisoformat(record.get("recorded_utc", ""))
                if stamp.tzinfo is None:
                    stamp = None
            except (ValueError, TypeError):
                stamp = None
            bounds = record.get("window_bounds", {})
            scale = record.get("actual_backing_scale")
            control = record.get("control", {})
            geometry = (
                isinstance(bounds, dict)
                and type(scale) in (int, float)
                and math.isfinite(scale)
                and scale in (1, 2)
                and type(bounds.get("width")) in (int, float)
                and type(bounds.get("height")) in (int, float)
                and bounds["width"] >= 300
                and bounds["height"] >= 200
                and any(
                    screenshot_fits(w, h, bounds, scale)
                    for w, h in (pictures[p] for p in imgs)
                )
            )
            valid = (
                geometry
                and stamp is not None
                and type(window_id) is int
                and window_id > 0
                and record.get("passed") is True
                and record.get("foreground_owned") is True
                and type(record.get("pid")) is int
                and record["pid"] > 0
                and record.get("actual_pointer_click") is True
                and isinstance(control, dict)
                and control.get("center_in_window") is True
                and control.get("ax_center_hit_matches") is True
                and control.get("title") in ("Add Folder", "Browse root 1")
            )
            if valid:
                session_root = "/".join(n.removeprefix(prefix).split("/")[:2])
                session = doc(session_root + "/session.json")
                exited = doc(session_root + "/exit.json")
                if (
                    session.get("pid") == record["pid"]
                    and session.get("binary_sha256") == gui
                    and session.get("source") == revision
                    and isinstance(session.get("identity"), str)
                    and bool(session["identity"])
                    and type(exited.get("exit_code")) is int
                    and exited["exit_code"] == 0
                    and exited.get("forced_cleanup") is False
                ):
                    captures.append(
                        (
                            record.get("title"),
                            scale,
                            (session_root, record["pid"]),
                            n,
                            stamp,
                        )
                    )
    for title in ("Scanner", "Settings"):
        check(
            "retina_pointer/" + title,
            any(t == title and s == 2 for t, s, *_ in captures),
        )
    per_pid = collections.defaultdict(list)
    for t, s, pid, n, stamp in captures:
        if t == "Scanner":
            per_pid[pid].append((stamp, s))
    transitions = []
    for samples in per_pid.values():
        scales = [s for _, s in sorted(samples)]
        collapsed = [s for i, s in enumerate(scales) if i == 0 or scales[i - 1] != s]
        transitions.append(
            any(collapsed[i : i + 3] == [2, 1, 2] for i in range(len(collapsed) - 2))
        )
    check("same_process_display_transitions", any(transitions))
    operator = doc("operator-report.json").get("cases", {})
    check(
        "operator_review",
        isinstance(operator, dict)
        and bool(operator)
        and all(
            isinstance(v, dict)
            and v.get("status") == "passed"
            and isinstance(v.get("notes"), str)
            and bool(v["notes"].strip())
            and bool(v.get("evidence"))
            and isinstance(v["evidence"], list)
            and all(
                isinstance(ref, str)
                and any(
                    n == prefix + ref or n.startswith(prefix + ref.rstrip("/") + "/")
                    for n in hashes
                )
                for ref in v["evidence"]
            )
            for v in operator.values()
        ),
    )
    check("no_ax_only_capture", not any(n.endswith("/ax-only.json") and any(part in n for part in ["/flow/", "/shutdown/", "/manual/"]) for n in hashes))
    check(
        "no_forced_cleanup", not any(n.endswith("/forced-cleanup.json") for n in hashes)
    )
    return {
        "schema": 1,
        "archive_integrity": "verified",
        "external_digest_checked": archive_sha256 is not None,
        "expected_source": revision,
        "members_hashed": len(hashes),
        "png_files": len(pictures),
        "coverage": coverage,
        "status": "ready_for_human_review"
        if all(v == "consistent" for v in coverage.values())
        else "needs_review",
        "human_review_required": True,
        "claims_are_self_reported": True,
        "manual_review": [
            "Readable/unobscured screenshots and visible focus",
            "Actual pointer and display transitions",
            "Source/build provenance and host authenticity",
            "Operator observations and privacy review",
        ],
        "formal_platform_qualification_complete": False,
        "runtime_convergence": False,
        "production_pin_adoption": False,
        "release_acceptance": False,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("archive", type=Path)
    parser.add_argument(
        "--archive-sha256", help="Trusted out-of-band digest, if available"
    )
    parser.add_argument(
        "--report",
        type=Path,
        required=True,
        help="New private JSON report; never overwritten",
    )
    parser.add_argument("--trusted-profile", type=Path, help="Locally reviewed rerun-kit profile; NEVER use a profile supplied inside returned evidence")
    args = parser.parse_args()
    if args.report.resolve().is_relative_to(Path(__file__).resolve().parents[1]):
        parser.error("Keep returned evidence reports outside the public checkout")
    code = 2
    try:
        result = verify(args.archive, args.archive_sha256, trusted_profile(args.trusted_profile) if args.trusted_profile else None)
        code = 0 if result["status"] == "ready_for_human_review" else 3
    except (
        Invalid,
        ValueError,
        KeyError,
        TypeError,
        OSError,
        zipfile.BadZipFile,
        zlib.error,
        EOFError,
        RuntimeError,
    ) as error:
        result = {
            "status": "invalid_package",
            "error": str(error),
            "human_review_required": True,
            "formal_platform_qualification_complete": False,
            "release_acceptance": False,
        }
    with args.report.open("x") as output:
        json.dump(result, output, indent=2)
        output.write("\n")
    print(result["status"])
    return code


if __name__ == "__main__":
    raise SystemExit(main())
