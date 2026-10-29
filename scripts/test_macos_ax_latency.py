import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('latency', Path(__file__).with_name('qualify-macos-ax-latency.py'))
latency = importlib.util.module_from_spec(spec)
spec.loader.exec_module(latency)


class LatencyGateTests(unittest.TestCase):
    def test_slow_or_missing_settings_never_passes(self):
        sample = dict(attribute_calls=224, window_count=1, median_ms=.1, snapshot_ms=40)
        self.assertFalse(latency.assess([]))
        self.assertFalse(latency.assess([sample]))
        settings = dict(sample, window_count=2, attribute_calls=432)
        self.assertTrue(latency.assess([sample, settings]))
        for bad in [dict(settings, median_ms=33), dict(settings, snapshot_ms=14000),
                    dict(settings, attribute_calls=0), dict(settings, median_ms=float('nan'))]:
            self.assertFalse(latency.assess([sample, bad]))
