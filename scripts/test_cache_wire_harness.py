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

class RegistryVerification(unittest.TestCase):
    def test_archive_hash_and_extracted_bytes_must_both_match(self):
        import hashlib
        import io
        import tarfile
        import tempfile
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / 'crate-1.0'
            root.mkdir()
            (root / 'source.rs').write_bytes(b'reviewed')
            archive = Path(temp) / 'crate.crate'
            with tarfile.open(archive, 'w:gz') as tar:
                info = tarfile.TarInfo('crate-1.0/source.rs')
                info.size = 8
                tar.addfile(info, io.BytesIO(b'reviewed'))
            digest = hashlib.sha256(archive.read_bytes()).hexdigest()
            wire.verify_registry_source(root, archive, digest)
            (root / '.cargo-ok').write_text('{}')
            wire.verify_registry_source(root, archive, digest)
            with self.assertRaises(ValueError):
                wire.verify_registry_source(root, archive, '0' * 64)
            (root / 'source.rs').write_bytes(b'modified')
            with self.assertRaises(ValueError):
                wire.verify_registry_source(root, archive, digest)
            (root / 'source.rs').write_bytes(b'reviewed')
            (root / 'extra.rs').write_text('unreviewed')
            with self.assertRaises(ValueError):
                wire.verify_registry_source(root, archive, digest)
