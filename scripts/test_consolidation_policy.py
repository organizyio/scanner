"""Regression tests for refusing unreviewed post-migration source drift."""
import ast
import hashlib
import tempfile
from pathlib import Path
import unittest


class SourcePolicyTests(unittest.TestCase):
    def test_changes_require_exact_reviewed_hash(self):
        source = Path(__file__).with_name('verify-consolidation.py').read_text()
        function = next(n for n in ast.parse(source).body if isinstance(n, ast.FunctionDef) and n.name == 'verify_current')
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'source.rs').write_bytes(b'new\r\n')
            policy = {'changes': {}}
            namespace = {'ROOT': root, 'policy': policy, 'hashlib': hashlib}
            exec(compile(ast.Module(body=[function], type_ignores=[]), '<policy-test>', 'exec'), namespace)
            verify = namespace['verify_current']
            with self.assertRaisesRegex(AssertionError, 'Unrecorded'):
                verify('source.rs', b'old\n')
            policy['changes']['source.rs'] = {'phase': 3, 'reason': 'bounded callbacks', 'tests': ['backpressure'], 'current_sha256': hashlib.sha256(b'new\n').hexdigest()}
            verify('source.rs', b'old\n')
            (root / 'source.rs').write_bytes(b'unreviewed\n')
            with self.assertRaisesRegex(AssertionError, 'review hash'):
                verify('source.rs', b'old\n')
            # Unmodified immutable content needs no exception.
            verify('source.rs', b'unreviewed\n')


    def test_vendor_rejects_unreviewed_source_and_extra_files(self):
        source = Path(__file__).with_name('verify-consolidation.py').read_text()
        function = next(n for n in ast.parse(source).body if isinstance(n, ast.FunctionDef) and n.name == 'verify_vendor')
        namespace = {'hashlib': hashlib}
        exec(compile(ast.Module(body=[function], type_ignores=[]), '<vendor-policy-test>', 'exec'), namespace)
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            path = directory / 'source.rs'
            path.write_bytes(b'reviewed public source\n')
            digest = hashlib.sha256(path.read_bytes()).hexdigest()
            register = {'original_files_sha256': {'source.rs': digest},
                        'current_files_sha256': {'source.rs': digest},
                        'modified_files': {}, 'added_files_sha256': {}}
            namespace['verify_vendor'](directory, register)
            path.write_bytes(b'unreviewed source\n')
            with self.assertRaisesRegex(AssertionError, 'Vendor review hash'):
                namespace['verify_vendor'](directory, register)
            path.write_bytes(b'reviewed public source\n')
            (directory / 'unregistered.rs').write_bytes(b'extra')
            with self.assertRaisesRegex(AssertionError, 'Unexpected vendor files'):
                namespace['verify_vendor'](directory, register)


if __name__ == '__main__':
    unittest.main()
