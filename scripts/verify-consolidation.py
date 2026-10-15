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

source = manifest['public_project_import']
subprocess.run(['git', 'merge-base', '--is-ancestor', source['head'], 'HEAD'], cwd=ROOT, check=True)
assert git('rev-parse', source['import_commit'] + ':tools/project-catalog') == source['tree']
assert git('rev-parse', source['head'] + '^{tree}') == source['tree']
for f, expected in manifest['retained_contract_files'].items():
    assert hashlib.sha256((ROOT / f).read_bytes().replace(b'\r\n', b'\n')).hexdigest() == expected, f
for path in git('ls-tree', '-r', '--name-only', source['head']).splitlines():
    if path.endswith('.go') or path in ('go.mod', 'go.sum', 'Makefile', 'build.sh'):
        expected = subprocess.check_output(['git', 'show', source['head'] + ':' + path], cwd=ROOT)
        actual = (ROOT / 'tools/project-catalog' / path).read_bytes()
        assert actual.replace(b'\r\n', b'\n') == expected.replace(b'\r\n', b'\n'), path
notices = json.loads((ROOT / 'licenses/upstream/manifest.json').read_text())
for notice in notices:
    assert hashlib.sha256((ROOT / notice['path']).read_bytes()).hexdigest() == notice['sha256'], notice['path']
for component in ('scan-engine', 'scan-fs'):
    assert not (ROOT / 'compat' / component / '.git').exists()
    assert not (ROOT / 'compat' / component / '.cursor').exists()
    assert not (ROOT / 'compat' / component / '.github').exists()
for remote in git('remote', '-v').splitlines():
    assert '/scan-engine' not in remote and '/scan-fs' not in remote, 'Private remote forbidden'
assert 'module github.com/meshory/scan-fs' in (ROOT / 'compat/scan-fs/go.mod').read_text()
assert 'module scan-projects' in (ROOT / 'tools/project-catalog/go.mod').read_text()
print('Public project ancestry, initial tree, retained contracts and isolated components verified')
