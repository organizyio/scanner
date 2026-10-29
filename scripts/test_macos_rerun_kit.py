import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

spec = importlib.util.spec_from_file_location('rerun', Path(__file__).with_name('prepare-macos-rerun-kit.py'))
kit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(kit)


class RerunKitTests(unittest.TestCase):
    def test_exact_revision_and_nonoverwrite_boundary(self):
        with tempfile.TemporaryDirectory() as directory:
            out = Path(directory) / 'new'
            revision = 'a' * 40
            def template(args, **kwargs):
                return (kit.ROOT / args[-1].split(':', 1)[1]).read_bytes()
            with patch.object(kit.subprocess, 'run') as ancestry, patch.object(kit.subprocess, 'check_output', side_effect=template):
                archive = kit.prepare(revision, out)
                ancestry.assert_called_once()
                self.assertEqual(ancestry.call_args.args[0][-2:], [revision, 'origin/main'])
                profile = json.loads((out / 'trusted-profile.json').read_text())
                self.assertEqual(profile['revision'], revision)
                with zipfile.ZipFile(archive) as z:
                    self.assertEqual(len(z.namelist()), 3)
                    source = z.read('scanner-macos-arm64-qualification-kit/qualify.py')
                    self.assertIn(revision.encode(), source)
                    self.assertNotIn(b'@SCANNER_REVISION@', source)
                    self.assertIn(b'qualify-macos-ax-latency.py', source)
                with self.assertRaises(ValueError):
                    kit.prepare(revision, out)
                with self.assertRaises(ValueError):
                    kit.prepare('main', Path(directory) / 'bad')
                with self.assertRaises(ValueError):
                    kit.prepare(revision, kit.ROOT / 'evidence')
