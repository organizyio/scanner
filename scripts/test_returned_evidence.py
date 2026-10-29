import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import stat
import struct
import sys
import tempfile
import unittest
from unittest.mock import patch
import zipfile
import zlib

spec = importlib.util.spec_from_file_location(
    "returned_evidence", Path(__file__).with_name("verify-returned-evidence.py")
)
v = importlib.util.module_from_spec(spec)
spec.loader.exec_module(v)


def png(w=600, h=400):
    def chunk(kind, body):
        return (
            struct.pack(">I", len(body))
            + kind
            + body
            + struct.pack(">I", zlib.crc32(kind + body) & 0xFFFFFFFF)
        )

    return (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0))
        + chunk(b"IDAT", zlib.compress((b"\0" + b"\0" * w * 3) * h))
        + chunk(b"IEND", b"")
    )


class ReturnedEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.path = Path(self.tmp.name) / "return.zip"
        self.kit = {
            "qualify.py": b"# synthetic helper, never executed\n",
            "display-probe.swift": b"// synthetic helper\n",
        }
        self.patch = patch.dict(
            v.KIT,
            {n: hashlib.sha256(b).hexdigest() for n, b in self.kit.items()},
            clear=True,
        )
        self.patch.start()
        self.addCleanup(self.patch.stop)
        self.docs, self.raw = {}, {}
        for name, data in self.kit.items():
            self.raw["handoff-kit/" + name] = data
        self.put(
            "host.json",
            {"machine": "arm64", "python_machine": "arm64", "root_user": False},
        )
        self.put("source-status.json", {"revision": v.REVISION, "status": ""})
        self.put(
            "build.json",
            {
                "source": v.REVISION,
                "binaries": {
                    n: {"sha256": "a" * 64, "architecture": "arm64"}
                    for n in [
                        "scan-gui",
                        "scan-cli",
                        "ax-helper",
                        "ax-observer",
                        "display-probe",
                    ]
                },
            },
        )
        self.put(
            "checks-summary.json",
            {
                "passed": True,
                "checks": {n: {"exit_code": 0, "passed": True} for n in v.CHECKS},
            },
        )
        self.put(
            "commands.json",
            [{"name": n, "exit_code": 0, "log": "logs/check.log"} for n in v.CHECKS],
        )
        self.raw["evidence/logs/check.log"] = b"synthetic check\n"
        self.put(
            "operator-report.json",
            {
                "cases": {
                    "visual": {
                        "status": "passed",
                        "notes": "Synthetic observations for verifier regression only",
                        "evidence": ["logs/check.log"],
                    }
                }
            },
        )
        self.put(
            "automated/a/accepted.json",
            {"passed": True, "source": v.REVISION, "gui_sha256": "a" * 64},
        )
        self.put(
            "automated/a/flow/report.json",
            dict(
                passed=True,
                source=v.REVISION,
                binary_sha256="a" * 64,
                forced_cleanup=False,
                native_quit_exit=0,
                quit_ms=20,
                **{
                    n: True
                    for n in [
                        "unicode",
                        "folder_error_recovery",
                        "profile_switch_restore",
                        "tree_keyboard",
                        "pause_resume_cancel",
                        "settings_restart_checkpoint",
                    ]
                },
            ),
        )
        runs = []
        for mode in ["attached", "detach-before-quit", "none"]:
            for scenario in ["idle"] * 3 + ["settings", "modal"]:
                runs.append(
                    dict(
                        observer_mode=mode,
                        scenario=scenario,
                        passed=True,
                        forced=False,
                        exit_code=0,
                        quit_ms=20,
                        observer_detach_ms=None,
                    )
                )
        runs.append(
            dict(
                observer_mode="attached",
                scenario="active",
                passed=True,
                forced=False,
                exit_code=0,
                quit_ms=20,
                observer_detach_ms=10,
            )
        )
        self.put(
            "automated/a/shutdown/summary.json",
            {
                "passed": True,
                "source": v.REVISION,
                "binary_sha256": "a" * 64,
                "runs": runs,
            },
        )
        self.put(
            "manual/s/session.json",
            {
                "pid": 123,
                "identity": "synthetic-process",
                "source": v.REVISION,
                "binary_sha256": "a" * 64,
            },
        )
        self.put("manual/s/exit.json", {"exit_code": 0, "forced_cleanup": False})
        for label, title, scale, stamp in [
            ("main-2x", "Scanner", 2, "01"),
            ("main-1x", "Scanner", 1, "02"),
            ("main-back-2x", "Scanner", 2, "03"),
            ("settings-2x", "Settings", 2, "04"),
        ]:
            self.capture(label, title, scale, stamp)

    def put(self, name, data):
        self.docs["evidence/" + name] = data

    def capture(self, label, title, scale, stamp):
        base = "manual/s/captures/" + label
        self.put(
            base + "/accepted.json",
            {
                "title": title,
                "pid": 123,
                "passed": True,
                "foreground_owned": True,
                "window_bounds": {"width": 300, "height": 200},
                "actual_backing_scale": scale,
                "actual_pointer_click": True,
                "recorded_utc": "2026-10-26T00:00:" + stamp + "+00:00",
                "window_id": 1,
                "control": {
                    "title": "Add Folder",
                    "center_in_window": True,
                    "ax_center_hit_matches": True,
                },
            },
        )
        self.raw["evidence/" + base + "/before/window-1.png"] = png(
            300 * scale, 200 * scale
        )

    def write(self, extra=None, bad_hash=False):
        docs = copy.deepcopy(self.docs)
        summary = {
            "source": v.REVISION,
            "origin": "https://github.com/organizyio/scanner.git",
            "source_validation_error": None,
            "host": docs["evidence/host.json"],
            "build": docs["evidence/build.json"],
            "checks": docs["evidence/checks-summary.json"],
            "operator": docs["evidence/operator-report.json"],
            "automated_runs": ["automated/a/accepted.json"],
            **{n: False for n in v.FLAGS},
        }
        docs["evidence/SUMMARY.json"] = summary
        index = [
            {
                "path": n.removeprefix("evidence/"),
                "width": v.png_geometry(b)[0],
                "height": v.png_geometry(b)[1],
                "sha256": hashlib.sha256(b).hexdigest(),
            }
            for n, b in self.raw.items()
            if n.endswith(".png")
        ]
        docs["evidence/screenshot-index.json"] = index
        files = {v.ROOT + n: b for n, b in self.raw.items()}
        files.update({v.ROOT + n: json.dumps(d).encode() for n, d in docs.items()})
        if extra:
            files.update(extra)
        manifest = {n: hashlib.sha256(b).hexdigest() for n, b in files.items()}
        if bad_hash:
            manifest[next(iter(manifest))] = "0" * 64
        files[v.ROOT + "FILES-SHA256.json"] = json.dumps(manifest).encode()
        with zipfile.ZipFile(self.path, "w", zipfile.ZIP_DEFLATED) as z:
            for n, b in files.items():
                z.writestr(n, b)
        return v.verify(self.path)

    def test_consistent_package_still_never_accepts_hardware_or_release(self):
        r = self.write()
        self.assertEqual(r["status"], "ready_for_human_review")
        self.assertTrue(r["human_review_required"])
        self.assertFalse(r["formal_platform_qualification_complete"])
        self.assertFalse(r["release_acceptance"])

    def test_cli_preserves_existing_reports_and_incomplete_package_never_accepts(self):
        self.docs["evidence/host.json"]["machine"] = "x86_64"
        self.write()
        report = Path(self.tmp.name) / "review.json"
        with patch.object(
            sys, "argv", ["verify", str(self.path), "--report", str(report)]
        ):
            self.assertEqual(v.main(), 3)
        before = report.read_bytes()
        self.assertFalse(json.loads(before)["formal_platform_qualification_complete"])
        with patch.object(
            sys, "argv", ["verify", str(self.path), "--report", str(report)]
        ):
            with self.assertRaises(FileExistsError):
                v.main()
        self.assertEqual(report.read_bytes(), before)

    def test_external_archive_digest_is_checked_when_supplied(self):
        self.write()
        checksum = hashlib.sha256(self.path.read_bytes()).hexdigest()
        self.assertTrue(v.verify(self.path, checksum)["external_digest_checked"])
        with self.assertRaisesRegex(v.Invalid, "External archive digest"):
            v.verify(self.path, "0" * 64)

    def test_archive_paths_are_not_extracted_or_executed(self):
        for path in [
            "../escape",
            "/escape",
            v.ROOT + "evidence/../escape",
            v.ROOT + "evidence/a\\b",
            v.ROOT + "evidence/C:evil",
            v.ROOT + "evidence/NUL.txt",
            v.ROOT + "evidence/.git/config",
        ]:
            with self.subTest(path=path), self.assertRaises(v.Invalid):
                self.write({path: b"no execution"})
        self.assertFalse((Path(self.tmp.name) / "escape").exists())

    def test_bad_manifest_and_unreviewed_kit_rejected(self):
        with self.assertRaisesRegex(v.Invalid, "hash mismatch"):
            self.write(bad_hash=True)
        self.raw["handoff-kit/qualify.py"] += b"changed"
        with self.assertRaisesRegex(v.Invalid, "Unreviewed"):
            self.write()

    def test_source_mismatch_and_dirty_source_remain_unqualified(self):
        self.docs["evidence/source-status.json"]["status"] = " M source.rs"
        self.assertEqual(
            self.write()["coverage"]["reviewed_source"], "missing_or_inconsistent"
        )

    def test_missing_checks_logs_or_false_pass_are_detected(self):
        self.docs["evidence/checks-summary.json"]["checks"].pop(
            "wrapper-real-engine-integration"
        )
        self.raw.pop("evidence/logs/check.log")
        r = self.write()
        self.assertEqual(
            r["coverage"]["build_and_cli_checks"], "missing_or_inconsistent"
        )
        self.assertEqual(r["coverage"]["check_logs_present"], "missing_or_inconsistent")

    def test_duplicate_shutdown_cases_cannot_replace_missing_cases(self):
        runs = self.docs["evidence/automated/a/shutdown/summary.json"]["runs"]
        runs[-1] = copy.deepcopy(runs[0])
        self.assertEqual(
            self.write()["coverage"]["automated/a"], "missing_or_inconsistent"
        )

    def test_failed_or_forced_late_quit_is_not_accepted(self):
        self.docs["evidence/automated/a/shutdown/summary.json"]["runs"][0][
            "quit_ms"
        ] = 8000
        self.put("automated/a/forced-cleanup.json", {"forced": True})
        r = self.write()
        self.assertEqual(r["coverage"]["automated/a"], "missing_or_inconsistent")
        self.assertEqual(r["coverage"]["no_forced_cleanup"], "missing_or_inconsistent")

    def test_screenshot_geometry_and_pointer_claims_need_evidence(self):
        self.docs["evidence/manual/s/captures/main-2x/accepted.json"][
            "actual_pointer_click"
        ] = False
        self.docs["evidence/manual/s/captures/main-back-2x/accepted.json"][
            "window_bounds"
        ]["width"] = 900
        r = self.write()
        self.assertEqual(
            r["coverage"]["retina_pointer/Scanner"], "missing_or_inconsistent"
        )
        self.assertEqual(
            r["coverage"]["same_process_display_transitions"], "missing_or_inconsistent"
        )

    def test_native_window_shadow_is_not_treated_as_wrong_display_scale(self):
        self.raw["evidence/manual/s/captures/main-2x/before/window-1.png"] = png(
            824, 624
        )
        self.assertEqual(
            self.write()["coverage"]["retina_pointer/Scanner"], "consistent"
        )

    def test_trusted_profile_requires_exact_local_authority(self):
        profile = Path(self.tmp.name) / "profile.json"
        valid = {"revision": v.REVISION, "kit": v.KIT}
        profile.write_text(json.dumps(valid))
        self.assertEqual(v.trusted_profile(profile), valid)
        for bad in [{}, dict(valid, revision="main"), dict(valid, kit={"evil.py": "a"*64}), dict(valid, extra=True)]:
            profile.write_text(json.dumps(bad))
            with self.assertRaises(v.Invalid):
                v.trusted_profile(profile)

    def test_ax_only_cannot_satisfy_visual_flow(self):
        self.put("automated/arm64-native/flow/ax-only.json", {"visual_qualification": False})
        self.write()
        result = v.verify(self.path)
        self.assertEqual(result["coverage"]["no_ax_only_capture"], "missing_or_inconsistent")

    def test_different_session_cannot_supply_missing_display_transition(self):
        self.docs["evidence/manual/s/session.json"]["binary_sha256"] = "b" * 64
        self.assertEqual(
            self.write()["coverage"]["same_process_display_transitions"],
            "missing_or_inconsistent",
        )

    def test_repeated_scale_samples_allow_real_ordered_transition(self):
        self.capture("extra-main-2x", "Scanner", 2, "00")
        self.assertEqual(
            self.write()["coverage"]["same_process_display_transitions"], "consistent"
        )

    def test_operator_pending_and_missing_references_remain_pending(self):
        self.docs["evidence/operator-report.json"]["cases"]["visual"]["status"] = (
            "pending"
        )
        self.assertEqual(self.write()["status"], "needs_review")

    def test_unsafe_zip_metadata_and_collision(self):
        self.write()
        with zipfile.ZipFile(self.path, "a") as z:
            info = zipfile.ZipInfo(v.ROOT + "evidence/link")
            info.create_system = 3
            info.external_attr = (stat.S_IFLNK | 0o777) << 16
            z.writestr(info, "../../escape")
        with self.assertRaisesRegex(v.Invalid, "Symlink"):
            v.verify(self.path)
        with self.assertRaisesRegex(v.Invalid, "colliding"):
            self.write({v.ROOT + "evidence/HOST.json": b"{}"})

    def test_embedded_nul_cannot_be_hidden_by_zipinfo_name_truncation(self):
        self.write({v.ROOT + "evidence/nameXtail.txt": b"data"})
        self.path.write_bytes(
            self.path.read_bytes().replace(b"nameXtail.txt", b"name\0tail.txt")
        )
        with self.assertRaisesRegex(v.Invalid, "Ambiguous ZIP member"):
            v.verify(self.path)

    def test_invalid_deflate_writes_invalid_package_report(self):
        self.write()
        data = bytearray(self.path.read_bytes())
        name_len, extra_len = struct.unpack("<HH", data[26:30])
        data[30 + name_len + extra_len] = 7  # forbidden deflate block type
        self.path.write_bytes(data)
        report = Path(self.tmp.name) / "invalid-review.json"
        with patch.object(
            sys, "argv", ["verify", str(self.path), "--report", str(report)]
        ):
            self.assertEqual(v.main(), 2)
        self.assertEqual(json.loads(report.read_text())["status"], "invalid_package")

    def test_duplicate_and_nonfinite_json_rejected(self):
        for b in [b'{"a":1,"a":2}', b'{"a":NaN}', b'{"a":1e999}']:
            with self.assertRaises(v.Invalid):
                v.load_json(b)

    def test_size_limits_and_invalid_png(self):
        with patch.object(v, "MAX_MEMBER", 32), self.assertRaises(v.Invalid):
            self.write()
        with patch.object(v, "MAX_FILES", 1), self.assertRaises(v.Invalid):
            self.write()
        with self.assertRaises(v.Invalid):
            v.png_geometry(png()[:-4])
        bad = bytearray(png())
        bad[30] ^= 1
        with self.assertRaises(v.Invalid):
            v.png_geometry(bytes(bad))


if __name__ == "__main__":
    unittest.main()
