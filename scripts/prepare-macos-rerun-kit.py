#!/usr/bin/env python3
"""Prepare a source-pinned public-source-only Mac rerun kit outside the checkout."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parents[1]


def prepare(revision, output):
    if not re.fullmatch('[0-9a-f]{40}', revision):
        raise ValueError('Use a full reviewed main commit SHA')
    subprocess.run(['git', 'merge-base', '--is-ancestor', revision, 'origin/main'], cwd=ROOT, check=True)
    output = output.resolve()
    if output.is_relative_to(ROOT) or output.exists():
        raise ValueError('Use a new output directory outside the checkout')
    files = {}
    for name in ['qualify.py', 'display-probe.swift', 'INSTRUCTIONS.md']:
        source = 'scripts/macos-rerun-kit/' + name
        data = subprocess.check_output(['git', 'show', revision + ':' + source], cwd=ROOT)
        files[name] = data.replace(b'@SCANNER_REVISION@', revision.encode())
    output.mkdir(parents=True)
    hashes = {n: hashlib.sha256(b).hexdigest() for n, b in files.items() if n.endswith(('.py', '.swift'))}
    profile = {'revision': revision, 'kit': hashes}
    (output / 'trusted-profile.json').write_text(json.dumps(profile, indent=2) + '\n')
    (output / 'SCANNER-MACOS-ARM64-QUALIFICATION.md').write_bytes(files.pop('INSTRUCTIONS.md'))
    files['FILES-SHA256.json'] = (json.dumps(hashes, indent=2) + '\n').encode()
    archive = output / 'scanner-macos-arm64-qualification-kit.zip'
    with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED) as z:
        for name, data in sorted(files.items()):
            z.writestr('scanner-macos-arm64-qualification-kit/' + name, data)
    archive.with_suffix('.zip.sha256').write_text(hashlib.sha256(archive.read_bytes()).hexdigest() + '  ' + archive.name + '\n')
    return archive


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--revision', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    print(prepare(args.revision, args.output))
