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
