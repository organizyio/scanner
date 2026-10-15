import importlib.util
import os
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("qualification", Path(__file__).with_name("platform-qualification.py"))
qualification = importlib.util.module_from_spec(spec)
spec.loader.exec_module(qualification)


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
                with qualification.denied_directory(denied):
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


if __name__ == "__main__":
    unittest.main()
