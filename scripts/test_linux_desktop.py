import importlib.util
import json
import tempfile
import threading
import time
import unittest
from pathlib import Path
from unittest.mock import patch

from linux_desktop import DesktopMonitor, probe, require_available

AVAILABLE = {"screensaver_active": False, "power_save_mode": 0}


class DesktopTests(unittest.TestCase):
    def test_locked_preflight_never_spawns_gui_or_private_session(self):
        spec = importlib.util.spec_from_file_location(
            "linux_session", Path(__file__).with_name("platform-linux-session.py")
        )
        session = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(session)
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "fresh"

            def factory(env, folder):
                return DesktopMonitor(
                    env,
                    folder,
                    inspect=lambda _: dict(AVAILABLE, screensaver_active=True),
                )

            with (
                patch.object(session, "DesktopMonitor", side_effect=factory),
                patch.object(session, "bus_status", return_value="(<false>,)"),
                patch.object(session.os, "getuid", return_value=1000, create=True),
                patch.object(session.subprocess, "Popen") as spawn,
            ):
                with self.assertRaisesRegex(RuntimeError, "locked"):
                    session.run(Path(tmp) / "unused-binary", root)
                spawn.assert_not_called()
            self.assertFalse((root / "session-report.json").exists())
            self.assertFalse((root / "worker-report.json").exists())

    def test_locked_off_unknown_and_unsupported_are_never_available(self):
        for state in [
            {},
            dict(AVAILABLE, screensaver_active=True),
            dict(AVAILABLE, screensaver_active=None),
            *[
                dict(AVAILABLE, power_save_mode=value)
                for value in [1, 2, 3, -1, None, False]
            ],
        ]:
            with self.subTest(state=state), self.assertRaises(RuntimeError):
                require_available(state)
        require_available(AVAILABLE)

    def test_probes_read_only_existing_bus_and_parse_strictly(self):
        env = {"DBUS_SESSION_BUS_ADDRESS": "synthetic-existing-bus"}
        with patch(
            "linux_desktop.subprocess.check_output",
            side_effect=["(false,)\n", "(<0>,)\n"],
        ) as call:
            self.assertEqual(probe(env), AVAILABLE)
            for arguments in call.call_args_list:
                self.assertIs(arguments.kwargs["env"], env)
                self.assertEqual(arguments.kwargs["timeout"], 2)
                self.assertNotIn("Set", arguments.args[0])
        with patch(
            "linux_desktop.subprocess.check_output", side_effect=["unknown", "(<0>,)"]
        ):
            with self.assertRaises(RuntimeError):
                probe(env)

    def test_preflight_rejects_locked_desktop_without_starting_monitor(self):
        with tempfile.TemporaryDirectory() as tmp:
            monitor = DesktopMonitor(
                {}, tmp, inspect=lambda _: dict(AVAILABLE, screensaver_active=True)
            )
            with self.assertRaisesRegex(RuntimeError, "locked"):
                monitor.start()
            self.assertIsNone(monitor.thread)
            monitor.close()
            self.assertFalse(
                json.loads((Path(tmp) / "desktop-state.json").read_text())[
                    "available_through_observed_run"
                ]
            )

    def test_lock_failure_is_not_erased_by_later_unlock(self):
        with tempfile.TemporaryDirectory() as tmp:
            states = iter(
                [AVAILABLE, dict(AVAILABLE, screensaver_active=True), AVAILABLE]
            )
            monitor = DesktopMonitor({}, tmp, inspect=lambda _: next(states, AVAILABLE))
            monitor.sample()
            monitor.check()
            monitor.sample()
            monitor.sample()
            with self.assertRaisesRegex(RuntimeError, "locked"):
                monitor.check()
            monitor.close()

    def test_slow_probe_cannot_block_native_deadline_polling(self):
        entered, release, checked = (
            threading.Event(),
            threading.Event(),
            threading.Event(),
        )
        calls = 0

        def inspect(_):
            nonlocal calls
            calls += 1
            if calls == 2:
                entered.set()
                release.wait(timeout=5)
            return AVAILABLE

        with tempfile.TemporaryDirectory() as tmp:
            monitor = DesktopMonitor({}, tmp, inspect=inspect, interval=0.01)
            monitor.start()
            thread = None
            try:
                self.assertTrue(entered.wait(timeout=2))
                thread = threading.Thread(
                    target=lambda: (monitor.check(), checked.set())
                )
                thread.start()
                self.assertTrue(
                    checked.wait(timeout=1), "desktop I/O blocked deadline supervision"
                )
            finally:
                release.set()
                if thread is not None:
                    thread.join(timeout=2)
                monitor.close()
            self.assertFalse(monitor.thread.is_alive())

    def test_stale_availability_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            monitor = DesktopMonitor({}, tmp, inspect=lambda _: AVAILABLE)
            monitor.sample()
            monitor.last_sample = time.monotonic() - 7
            with self.assertRaisesRegex(RuntimeError, "stale"):
                monitor.check()
            monitor.close()


if __name__ == "__main__":
    unittest.main()
