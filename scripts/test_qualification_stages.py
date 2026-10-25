import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path

import qualification_stages as stages


class NativeDeadlineTests(unittest.TestCase):
    def test_blocked_worker_is_detected_before_it_returns(self):
        with tempfile.TemporaryDirectory() as tmp:
            worker = subprocess.Popen(
                [
                    sys.executable,
                    "-c",
                    "import qualification_stages as s,time,sys;"
                    "s.begin(sys.argv[1],'owned','quit',.1);time.sleep(30)",
                    tmp,
                ],
                cwd=Path(__file__).parent,
            )
            try:
                deadline = time.monotonic() + 5
                while not stages.expired(tmp, "owned"):
                    self.assertLess(time.monotonic(), deadline)
                    time.sleep(0.01)
                self.assertIsNone(worker.poll())
            finally:
                worker.kill()
                worker.wait(timeout=5)

    def test_late_completion_cannot_hide_between_polls(self):
        with tempfile.TemporaryDirectory() as tmp:
            value = stages.begin(tmp, "owned", "quit")
            value["finish_ns"] = value["start_ns"] + value["limit_ns"]
            stages.write_stage(tmp, value)
            self.assertTrue(stages.expired(tmp, "owned"))

    def test_completed_stage_does_not_expire_later(self):
        with tempfile.TemporaryDirectory() as tmp:
            value = stages.begin(tmp, "owned", "detach")
            self.assertGreaterEqual(stages.finish(tmp, value), 0)
            self.assertFalse(
                stages.expired(tmp, "owned", value["start_ns"] + 20_000_000_000)
            )

    def test_nonce_limit_and_reversed_clock_fail_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            value = stages.begin(tmp, "owned", "quit")
            with self.assertRaises(ValueError):
                stages.expired(tmp, "other-run")
            for invalid in (
                dict(value, limit_ns=9_000_000_000),
                dict(value, finish_ns=value["start_ns"] - 1),
            ):
                stages.write_stage(tmp, invalid)
                with self.assertRaises(ValueError):
                    stages.expired(tmp, "owned")


if __name__ == "__main__":
    unittest.main()
