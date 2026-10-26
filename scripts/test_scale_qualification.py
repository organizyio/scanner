import importlib.util
from pathlib import Path
import sys
import tempfile
import time
import unittest

spec = importlib.util.spec_from_file_location(
    "scale_qualification", Path(__file__).with_name("qualify-scale.py")
)
scale = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scale)


class ScaleQualificationTests(unittest.TestCase):
    def test_owned_child_timeout_is_bounded_and_reaped(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            started = time.monotonic()
            with self.assertRaises(TimeoutError):
                scale.run_probe(
                    [sys.executable, "-c", "import time;time.sleep(60)"],
                    root / "out",
                    root / "err",
                    0.1,
                )
            self.assertLess(time.monotonic() - started, 10)
            # Independent following child works after failed/terminated child.
            result = scale.run_probe(
                [sys.executable, "-c", 'print("{}")'],
                root / "after",
                root / "after.err",
                10,
            )
            self.assertIn("peak_rss_bytes", result)
            if result["peak_rss_bytes"] is not None:
                self.assertGreater(result["peak_rss_bytes"], 0)

    def test_failing_probe_cannot_be_reported_as_passed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with self.assertRaises(RuntimeError):
                scale.run_probe(
                    [sys.executable, "-c", "raise SystemExit(4)"],
                    root / "out",
                    root / "err",
                    10,
                )

    def test_recovery_and_inventory_are_required_not_just_exit_zero(self):
        value = {
            "passed": True,
            "mode": "cancel",
            "walk_mode": "full",
            "fixture_files": 20,
            "files_delivered": 5,
            "bytes_delivered": 1280,
            "expected_failure_observed": False,
            "recovery_inventory_verified": True,
            "elapsed_ms": 20.0,
            "first_record_ms": 1.0,
            "cancel_return_ms": 1.0,
            "release_acceptance": False,
        }
        scale.validate(value, "cancel", "full", 20, 5)
        for key, wrong in [
            ("files_delivered", 6),
            ("recovery_inventory_verified", False),
            ("first_record_ms", float("nan")),
            ("elapsed_ms", 0.0),
            ("release_acceptance", True),
        ]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                scale.validate(dict(value, **{key: wrong}), "cancel", "full", 20, 5)

    def test_mixed_fixture_has_bounded_reproducible_classes_and_real_documents(self):
        import hashlib
        import io
        import struct
        import zipfile
        import xml.etree.ElementTree as ET
        from scale_fixtures import recipes, specification

        bodies = recipes("mixed")
        self.assertEqual(
            [v[0] for v in bodies],
            ["empty", "medium", "large", "huge", "image", "document", "pdf", "tiny"],
        )
        self.assertEqual([len(v[2]) for v in bodies[:4]], [0, 65536, 1048576, 8388608])
        self.assertEqual(bodies, recipes("mixed"))
        self.assertEqual(struct.unpack(">II", bodies[4][2][16:24]), (1024, 768))
        with zipfile.ZipFile(io.BytesIO(bodies[5][2])) as archive:
            xml = archive.read("docProps/core.xml")
            element = ET.fromstring(xml)
            self.assertIn("Scanner Synthetic", [node.text for node in element])
            self.assertGreater(len(xml), 512 << 10)
        self.assertIn(b"/Count 32", bodies[6][2])
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            spec = scale.fixture(root, 128, "mixed")
            self.assertEqual(spec, specification(128, "mixed", bodies))
            for row in spec["classes"]:
                files = list(root.rglob("file-*." + row["extension"]))
                self.assertTrue(
                    any(
                        hashlib.sha256(p.read_bytes()).hexdigest() == row["sha256"]
                        for p in files
                    )
                )
            self.assertEqual(sum(spec["class_counts"].values()), 128)
            self.assertLess(spec["total_bytes"], 16 << 20)

    def test_mixed_validation_requires_extraction_classes_and_exact_bytes(self):
        from scale_fixtures import specification

        spec = specification(128, "mixed")
        value = dict(
            passed=True,
            mode="fast",
            walk_mode="full",
            fixture_files=128,
            files_delivered=128,
            bytes_delivered=spec["total_bytes"],
            delivered_by_class=spec["class_counts"],
            profile="mixed",
            extraction_enabled=True,
            expected_unsupported_image_records=1,
            expected_failure_observed=False,
            recovery_inventory_verified=False,
            elapsed_ms=20.0,
            first_record_ms=1.0,
            release_acceptance=False,
        )
        scale.validate(value, "fast", "full", 128, 64, spec)
        for key, wrong in [
            ("profile", "small"),
            ("extraction_enabled", False),
            ("bytes_delivered", 128 * 256),
            ("delivered_by_class", {"tiny": 128}),
        ]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                scale.validate(
                    dict(value, **{key: wrong}), "fast", "full", 128, 64, spec
                )

    def test_fixture_is_exact_and_multiroot_without_personal_data(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            scale.fixture(root, 100)
            files = list(root.rglob("*.txt"))
            self.assertEqual(len(files), 100)
            self.assertTrue(all(p.read_bytes() == b"x" * 256 for p in files))
            self.assertEqual(
                {p.relative_to(root).parts[0] for p in files}, {"alpha", "unicode-β"}
            )


if __name__ == "__main__":
    unittest.main()
