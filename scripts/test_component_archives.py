import hashlib
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

spec = importlib.util.spec_from_file_location('components', Path(__file__).with_name('build-components.py'))
components = importlib.util.module_from_spec(spec)
spec.loader.exec_module(components)


class ComponentArchives(unittest.TestCase):
    def test_zip_retains_nested_notices_with_pre_1980_timestamps(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            out = root / 'dist/wrapper/windows-amd64'
            notice = out / 'third-party/example/LICENSE'
            notice.parent.mkdir(parents=True)
            notice.write_bytes(b'upstream notice\n')
            os.utime(notice, (0, 0))
            binary = out / 'scan-fs.exe'
            binary.write_bytes(b'fixture binary')
            with patch.object(components, 'ROOT', root), patch.object(components.platform, 'system', return_value='Windows'):
                components.package(out)
            archive = root / 'dist/wrapper-windows-amd64.zip'
            with zipfile.ZipFile(archive) as z:
                self.assertEqual(z.read('wrapper-windows-amd64/third-party/example/LICENSE'), notice.read_bytes())
                self.assertEqual(z.read('wrapper-windows-amd64/scan-fs.exe'), binary.read_bytes())
            self.assertEqual(archive.with_name(archive.name + '.sha256').read_text().split()[0], hashlib.sha256(archive.read_bytes()).hexdigest())

    def test_notice_can_be_copied_again_from_readonly_module_cache(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / 'source'
            source.write_bytes(b'license')
            source.chmod(0o444)
            destination = Path(directory) / 'dist/LICENSE'
            components.copy_notice(source, destination)
            destination.chmod(0o444)
            components.copy_notice(source, destination)
            self.assertEqual(source.read_bytes(), destination.read_bytes())
            source.chmod(0o644)


if __name__ == '__main__':
    unittest.main()
