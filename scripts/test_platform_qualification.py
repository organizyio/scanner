import importlib.util
import os
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("qualification", Path(__file__).with_name("platform-qualification.py"))
qualification = importlib.util.module_from_spec(spec)
spec.loader.exec_module(qualification)
spec_x11 = importlib.util.spec_from_file_location("qualification_x11", Path(__file__).with_name("platform-x11.py"))
x11 = importlib.util.module_from_spec(spec_x11)
spec_x11.loader.exec_module(x11)


class QualificationTests(unittest.TestCase):
    def test_fixture_isolates_all_settings_without_changing_parent_environment(self):
        original = os.environ.copy()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            env = qualification.prepare(root)
            for key in ["HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME"]:
                self.assertTrue(Path(env[key]).is_relative_to(root))
            self.assertIn("Qualification", (qualification.settings_directory(root) / "settings.yaml").read_text())
            with self.assertRaises(ValueError):
                qualification.prepare(root)
        self.assertEqual(original, dict(os.environ))

    def test_permission_restored_on_failure(self):
        with tempfile.TemporaryDirectory() as tmp:
            denied = Path(tmp) / "denied"
            with self.assertRaisesRegex(RuntimeError, "synthetic"):
                with qualification.denied_directory(denied) as actual_denial:
                    if os.name == "nt":
                        self.assertTrue(actual_denial, "Windows fixture must genuinely deny access")
                    raise RuntimeError("synthetic")
            self.assertEqual((denied / "private-fixture.txt").read_text(), "synthetic denied content")

    def test_nonempty_directory_without_marker_is_never_overwritten(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            original = root / "settings.yaml"
            original.write_text("unrelated local content")
            with self.assertRaisesRegex(ValueError, "empty fixture"):
                qualification.prepare(root)
            self.assertEqual(original.read_text(), "unrelated local content")

    def test_unrelated_stderr_or_failed_exit_never_qualifies_denial(self):
        affected = "/synthetic/locked.txt"
        payload = {"files": [{"path": affected, "hash_status": "error"}]}
        for exit_code, stderr in [(0, "unrelated locale warning"), (1, affected + ": read denied")]:
            result = {"exit_code": exit_code, "valid_envelope": True}
            qualification.assess_denial("locked-file", result, payload, stderr, affected, True, windows=True)
            self.assertEqual(result["status"], "failed")

    def test_locked_read_requires_affected_hash_error_and_associated_diagnostic(self):
        affected = "/synthetic/locked.txt"
        for status, expected in [("ok", "failed"), ("error", "passed")]:
            result = {"exit_code": 0, "valid_envelope": True}
            payload = {"files": [{"path": affected, "hash_status": status}]}
            qualification.assess_denial("locked-file", result, payload, affected + ": sharing denied", affected, True, windows=True)
            self.assertEqual(result["status"], expected)

    def test_focus_loss_prevents_every_event_in_sequence(self):
        delivered = []
        def denied():
            raise RuntimeError("focus lost")
        with self.assertRaisesRegex(RuntimeError, "focus lost"):
            x11.checked_sequence(denied, [lambda: delivered.append("input")])
        self.assertEqual(delivered, [])
        x11.checked_sequence(lambda: None, [lambda: delivered.append("input")])
        self.assertEqual(delivered, ["input"])

    def test_focus_requires_selected_window_or_descendant(self):
        parents = {300: 200, 200: 100, 100: 0, 400: 0}
        self.assertTrue(x11.descendant_of(300, 200, lambda v: parents[v]))
        self.assertFalse(x11.descendant_of(400, 200, lambda v: parents[v]))
        self.assertFalse(x11.descendant_of(1, 200, lambda v: parents[v]))

    def test_coordinates_use_actual_settings_window_geometry(self):
        x11.validate_point(799, 619, 800, 620)
        with self.assertRaises(ValueError):
            x11.validate_point(900, 650, 800, 620)


if __name__ == "__main__":
    unittest.main()
