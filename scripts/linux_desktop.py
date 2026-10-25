"""Observe the existing GNOME desktop; never unlock it or inhibit its policies.

The qualification parent polls cached samples so desktop D-Bus calls cannot
block native-action deadline supervision. Unknown/unavailable state fails closed.
"""

import json
import re
import subprocess
import threading
import time
from pathlib import Path


def require_available(state):
    if state.get("screensaver_active") is not False:
        raise RuntimeError(
            "Desktop locked, screen saver active or state unknown; GUI qualification pending"
        )
    if type(state.get("power_save_mode")) is not int or state["power_save_mode"] != 0:
        raise RuntimeError(
            "Display is not confirmed powered on; GUI qualification pending"
        )


def probe(env):
    def call(arguments):
        return subprocess.check_output(
            ["gdbus", "call", "--session", *arguments],
            env=env,
            text=True,
            stderr=subprocess.PIPE,
            timeout=2,
        ).strip()

    locked = call(
        [
            "--dest",
            "org.gnome.ScreenSaver",
            "--object-path",
            "/org/gnome/ScreenSaver",
            "--method",
            "org.gnome.ScreenSaver.GetActive",
        ]
    )
    power = call(
        [
            "--dest",
            "org.gnome.Mutter.DisplayConfig",
            "--object-path",
            "/org/gnome/Mutter/DisplayConfig",
            "--method",
            "org.freedesktop.DBus.Properties.Get",
            "org.gnome.Mutter.DisplayConfig",
            "PowerSaveMode",
        ]
    )
    if locked not in ("(true,)", "(false,)"):
        raise RuntimeError("Unrecognized GNOME screen-saver state")
    match = re.fullmatch(r"\(<(-?\d+)>,\)", power)
    if not match:
        raise RuntimeError("Unrecognized GNOME display-power state")
    return {"screensaver_active": locked == "(true,)", "power_save_mode": int(match[1])}


class DesktopMonitor:
    def __init__(self, env, root, inspect=probe, interval=1):
        self.env, self.root, self.inspect, self.interval = (
            env,
            Path(root),
            inspect,
            interval,
        )
        self.stop = threading.Event()
        self.lock = threading.Lock()
        self.samples = []
        self.failure = None
        self.last_sample = 0
        self.thread = None

    def sample(self):
        try:
            state = self.inspect(self.env)
            require_available(state)
            failure = None
        except Exception as error:
            state = locals().get("state", {})
            failure = str(error)
        with self.lock:
            self.last_sample = time.monotonic()
            self.samples.append(
                dict(
                    state,
                    monotonic_seconds=self.last_sample,
                    available=failure is None,
                    error=failure,
                )
            )
            # A later valid sample cannot erase a lock/power failure during a run.
            if failure is not None:
                self.failure = failure
        return state

    def check(self):
        with self.lock:
            if self.failure is not None:
                raise RuntimeError(self.failure)
            if time.monotonic() - self.last_sample > 6:
                raise RuntimeError(
                    "Desktop availability monitor is stale; qualification pending"
                )

    def start(self):
        self.sample()
        self.check()

        def monitor():
            while not self.stop.wait(self.interval):
                self.sample()

        self.thread = threading.Thread(target=monitor, name="owned-desktop-state")
        self.thread.start()

    def close(self):
        self.stop.set()
        if self.thread is not None:
            self.thread.join()  # Two subprocess probes, each bounded by two seconds.
        self.sample()
        with self.lock:
            report = {
                "backend": "GNOME",
                "sampling_interval_seconds": self.interval,
                "samples": list(self.samples),
                "available_through_observed_run": self.failure is None,
                "desktop_policy_changed": False,
            }
        (self.root / "desktop-state.json").write_text(
            json.dumps(report, indent=2) + "\n"
        )
