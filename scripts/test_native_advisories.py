"""Regression coverage for bounded native maintenance dispositions."""
import copy
from datetime import date
import importlib.util
from pathlib import Path
import tomllib
import unittest

SPEC = importlib.util.spec_from_file_location('native_policy', Path(__file__).with_name('check-native-advisories.py'))
POLICY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(POLICY)


class NativePolicyTests(unittest.TestCase):
    def setUp(self):
        load = lambda p: tomllib.loads(p.read_text())
        self.root = load(POLICY.ROOT / 'deny.toml')
        self.native = load(POLICY.ROOT / 'tools/project-catalog/native-a11y/deny.toml')
        self.lock = load(POLICY.ROOT / 'tools/project-catalog/native-a11y/Cargo.lock')

    def verify(self, today=date(2026, 10, 24)):
        POLICY.verify(self.root, self.native, self.lock, today)

    def test_exact_reviewed_policy(self):
        self.verify()

    def test_expired_review_is_rejected(self):
        with self.assertRaisesRegex(ValueError, 'expired'):
            self.verify(POLICY.REVIEW_BEFORE)

    def test_other_relaxation_is_rejected(self):
        self.native['licenses']['allow'].append('bzip2-1.0.6')
        with self.assertRaisesRegex(ValueError, 'otherwise equal'):
            self.verify()

    def test_dependency_change_is_rejected(self):
        next(p for p in self.lock['package'] if p['name'] == 'zvariant')['version'] = '5.15.1'
        with self.assertRaisesRegex(ValueError, 'version changed'):
            self.verify()

    def test_missing_or_duplicate_disposition_is_rejected(self):
        self.native['advisories']['ignore'].append(copy.deepcopy(self.native['advisories']['ignore'][0]))
        with self.assertRaisesRegex(ValueError, 'exactly'):
            self.verify()


if __name__ == '__main__':
    unittest.main()
