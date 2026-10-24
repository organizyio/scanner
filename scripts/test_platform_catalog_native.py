import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("native_catalog", Path(__file__).with_name("platform-catalog-native.py"))
native = importlib.util.module_from_spec(spec)
spec.loader.exec_module(native)


spec_signal = importlib.util.spec_from_file_location("windows_signal", Path(__file__).with_name("platform-windows-signal.py"))
windows_signal = importlib.util.module_from_spec(spec_signal)
spec_signal.loader.exec_module(windows_signal)

spec_linux = importlib.util.spec_from_file_location("linux_session", Path(__file__).with_name("platform-linux-session.py"))
linux_session = importlib.util.module_from_spec(spec_linux)
spec_linux.loader.exec_module(linux_session)

class NativeCatalogTests(unittest.TestCase):
    def test_renderer_selection_is_explicit_and_child_only(self):
        parent = {"DISPLAY": ":0", "LIBGL_ALWAYS_SOFTWARE": "1", "vblank_mode": "1"}
        chosen = linux_session.renderer_environment(parent, "hardware-no-vsync")
        self.assertNotIn("LIBGL_ALWAYS_SOFTWARE", chosen)
        self.assertEqual(chosen["vblank_mode"], "0")
        self.assertEqual(parent["LIBGL_ALWAYS_SOFTWARE"], "1")
        self.assertEqual(parent["vblank_mode"], "1")
        self.assertEqual(linux_session.renderer_environment(parent, "software")["LIBGL_ALWAYS_SOFTWARE"], "1")
        self.assertEqual(linux_session.renderer_environment(parent, "default"), parent)
        with self.assertRaises(ValueError):
            linux_session.renderer_environment(parent, "implicit-driver-change")

    def test_linux_activation_cannot_use_host_or_persistent_settings(self):
        linux_session.require_private_bus({"GSETTINGS_BACKEND": "memory", "DBUS_SESSION_BUS_ADDRESS": "private"}, "host")
        with self.assertRaisesRegex(RuntimeError, "private"):
            linux_session.require_private_bus({"GSETTINGS_BACKEND": "memory", "DBUS_SESSION_BUS_ADDRESS": "private"}, None)
        for env in [{"GSETTINGS_BACKEND": "memory", "DBUS_SESSION_BUS_ADDRESS": "host"},
                    {"GSETTINGS_BACKEND": "dconf", "DBUS_SESSION_BUS_ADDRESS": "private"},
                    {"GSETTINGS_BACKEND": "memory"}]:
            with self.assertRaisesRegex(RuntimeError, "private"):
                linux_session.require_private_bus(env, "host")

    def test_unrelated_console_member_prevents_control_event(self):
        windows_signal.require_console_members({100, 101}, {100, 101})
        for actual in [{100, 101, 999}, {100}, set()]:
            with self.assertRaisesRegex(RuntimeError, "membership"):
                windows_signal.require_console_members(actual, {100, 101})

    def test_skipped_missing_or_failed_native_case_never_passes(self):
        full = [{"Test": test, "Action": "pass"} for test in sorted(native.EXPECTED)]
        for values, code in [(full[:-1], 0), (full, 1),
                             ([*full[:-1], {"Test": full[-1]["Test"], "Action": "skip"}], 0),
                             ([*full[:-1], {"Test": full[-1]["Test"], "Action": "fail"}], 0)]:
            _, passed = native.summarize("\n".join(map(json.dumps, values)), code)
            self.assertFalse(passed)
        _, passed = native.summarize("\n".join(map(json.dumps, full)), 0)
        self.assertTrue(passed)

    def test_evidence_inside_checkout_is_refused_before_writes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with self.assertRaisesRegex(ValueError, "outside"):
                native.run(root, root / "evidence")
            self.assertFalse((root / "evidence").exists())


if __name__ == "__main__":
    unittest.main()
