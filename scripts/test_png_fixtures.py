"""Fixture provenance and repeatable standard-library generation."""

import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


class PngFixtures(unittest.TestCase):
    def test_regeneration_matches_every_tracked_byte_and_hash(self):
        self.check_generator("generate-png-fixtures.py")

    def test_limit_headers_are_reproducible(self):
        self.check_generator("generate-png-limit-fixtures.py")

    def test_metadata_controls_are_reproducible(self):
        self.check_generator("generate-metadata-fixtures.py")

    def check_generator(self, name):
        source = Path(__file__).with_name(name)
        spec = importlib.util.spec_from_file_location("png_fixtures", source)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        tracked = module.ROOT
        with tempfile.TemporaryDirectory() as tmp:
            module.ROOT = Path(tmp)
            module.main()
            manifest = json.loads((tracked / "manifest.json").read_text())
            self.assertEqual(
                set(manifest),
                {p.name for p in tracked.iterdir() if p.name != "manifest.json"},
            )
            for name, row in manifest.items():
                with self.subTest(name=name):
                    self.assertEqual(
                        (tracked / name).read_bytes(), (module.ROOT / name).read_bytes()
                    )
                    self.assertEqual(
                        hashlib.sha256((tracked / name).read_bytes()).hexdigest(),
                        row["sha256"],
                    )
            self.assertEqual(
                (tracked / "manifest.json").read_bytes(),
                (module.ROOT / "manifest.json").read_bytes(),
            )


if __name__ == "__main__":
    unittest.main()
