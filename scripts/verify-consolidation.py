#!/usr/bin/env python3
"""Public, deterministic history and source-integrity checks. No private refs embedded."""
import hashlib
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
manifest = json.loads((ROOT / 'docs/consolidation/manifest.json').read_text())

def git(*args):
    return subprocess.check_output(['git', *args], cwd=ROOT, text=True).strip()

policy = json.loads((ROOT / 'docs/follow-up/changes.json').read_text())
baseline = policy['migration_baseline']
subprocess.run(['git', 'merge-base', '--is-ancestor', baseline, 'HEAD'], cwd=ROOT, check=True)

def baseline_bytes(path):
    return subprocess.check_output(['git', 'show', baseline + ':' + path], cwd=ROOT).replace(b'\r\n', b'\n')

def verify_current(path, initial):
    actual = (ROOT / path).read_bytes().replace(b'\r\n', b'\n')
    if actual != initial:
        approval = policy['changes'].get(path)
        assert approval and approval.get('phase') in range(1, 6) and approval.get('reason') and approval.get('tests'), 'Unrecorded post-consolidation change: ' + path
        assert hashlib.sha256(actual).hexdigest() == approval.get('current_sha256'), 'Post-consolidation review hash: ' + path

source = manifest['public_project_import']
subprocess.run(['git', 'merge-base', '--is-ancestor', source['head'], 'HEAD'], cwd=ROOT, check=True)
assert git('rev-parse', source['import_commit'] + ':tools/project-catalog') == source['tree']
assert git('rev-parse', source['head'] + '^{tree}') == source['tree']
for f, expected in manifest['retained_contract_files'].items():
    initial = baseline_bytes(f)
    assert hashlib.sha256(initial).hexdigest() == expected, 'Immutable migration baseline: ' + f
    verify_current(f, initial)
for path in git('ls-tree', '-r', '--name-only', source['head']).splitlines():
    if path.endswith('.go') or path in ('go.mod', 'go.sum', 'Makefile', 'build.sh'):
        expected = subprocess.check_output(['git', 'show', source['head'] + ':' + path], cwd=ROOT)
        prefixed = 'tools/project-catalog/' + path
        initial = baseline_bytes(prefixed)
        adaptation = manifest.get('project_catalog_source_adaptations', {}).get(path)
        if adaptation:
            assert hashlib.sha256(expected.replace(b'\r\n', b'\n')).hexdigest() == adaptation['original_sha256']
            assert hashlib.sha256(initial).hexdigest() == adaptation['current_sha256']
        else:
            assert initial == expected.replace(b'\r\n', b'\n'), path
        verify_current(prefixed, initial)
notices = json.loads((ROOT / 'licenses/upstream/manifest.json').read_text())
for notice in notices:
    assert hashlib.sha256((ROOT / notice['path']).read_bytes()).hexdigest() == notice['sha256'], notice['path']
def verify_vendor(directory, register):
    original = register['original_files_sha256']
    current = register['current_files_sha256']
    modified = register['modified_files']
    added = register['added_files_sha256']
    assert set(modified) <= set(original), 'Unknown upstream modification'
    assert set(current) == set(original) | set(added), 'Unregistered vendor files'
    assert set(added) == set(current) - set(original), 'Invalid added vendor files'
    entries = list(directory.rglob('*'))
    assert not any(p.is_symlink() for p in entries), 'Vendor symlink forbidden'
    assert all(p.is_file() or p.is_dir() for p in entries), 'Vendor special file forbidden'
    actual = {p.relative_to(directory).as_posix() for p in entries if p.is_file()}
    assert actual == set(current), 'Unexpected vendor files'
    for name, expected in current.items():
        path = directory / name
        assert not path.is_symlink(), 'Vendor symlink forbidden: ' + name
        assert hashlib.sha256(path.read_bytes()).hexdigest() == expected, 'Vendor review hash: ' + name
        if name in original and name not in modified:
            assert expected == original[name], 'Unrecorded upstream change: ' + name
        if name in added:
            assert expected == added[name], 'Added vendor hash: ' + name

vendor = ROOT / 'tools/project-catalog/native-a11y/vendor'
verify_vendor(vendor / 'accesskit_unix', json.loads((vendor / 'provenance.json').read_text()))
for component in ('scan-engine', 'scan-fs'):
    assert not (ROOT / 'compat' / component / '.git').exists()
    assert not (ROOT / 'compat' / component / '.cursor').exists()
    assert not (ROOT / 'compat' / component / '.github').exists()
for remote in git('remote', '-v').splitlines():
    assert '/scan-engine' not in remote and '/scan-fs' not in remote, 'Private remote forbidden'
assert 'module github.com/meshory/scan-fs' in (ROOT / 'compat/scan-fs/go.mod').read_text()
assert 'module scan-projects' in (ROOT / 'tools/project-catalog/go.mod').read_text()
print('Public project ancestry, initial tree, retained contracts and isolated components verified')
