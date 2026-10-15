import importlib.util
from pathlib import Path
import os
import sys
import tempfile
import time
import unittest

spec = importlib.util.spec_from_file_location('capture', Path(__file__).with_name('capture-contracts.py'))
capture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(capture)


class CaptureDeadlineTests(unittest.TestCase):
    def test_stalled_stdout_is_terminated(self):
        start = time.monotonic()
        with tempfile.TemporaryFile() as stderr:
            with self.assertRaises(TimeoutError):
                capture.read_process([sys.executable, '-c', 'import time; time.sleep(60)'], os.environ, stderr, timeout=0.1)
        self.assertLess(time.monotonic() - start, 5)

    def test_nonzero_exit_and_records_are_captured(self):
        with tempfile.TemporaryFile() as stderr:
            first, rest, code, *_ = capture.read_process([sys.executable, '-c', 'print("record");raise SystemExit(2)'], os.environ, stderr)
        self.assertEqual((first.splitlines(), rest, code), ([b'record'], b'', 2))


if __name__ == '__main__':
    unittest.main()
