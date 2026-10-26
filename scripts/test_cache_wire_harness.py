import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location(
    "cache_wire", Path(__file__).with_name("test-native-cache-signals.py")
)
wire = importlib.util.module_from_spec(spec)
spec.loader.exec_module(wire)


class CacheWireHarness(unittest.TestCase):
    def test_instrumentation_retains_production_and_negative_control_is_narrow(self):
        source = "before\n                    &(body,),\nafter\n"
        self.assertEqual(wire.instrument(source, "TEST"), source + "\nTEST")
        self.assertEqual(
            wire.instrument(source, "TEST", True),
            "before\n                    body,\nafter\n\nTEST",
        )
        for changed in ["no seam", source + source]:
            with self.assertRaises(ValueError):
                wire.instrument(changed, "TEST")

    def test_duplicate_or_drifted_sources_cannot_hide_in_package_map(self):
        a = {"name": "pkg", "version": "1", "source": "registry", "checksum": "a"}
        self.assertEqual(
            wire.packages({"package": [a]}), {("pkg", "1"): ("registry", "a")}
        )
        with self.assertRaises(ValueError):
            wire.packages({"package": [a, dict(a, source="foreign")]})


if __name__ == "__main__":
    unittest.main()
