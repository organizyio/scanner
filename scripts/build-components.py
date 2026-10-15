#!/usr/bin/env python3
"""Build isolated migration components from local reviewed source; never download engines."""
import argparse
import hashlib
import os
from pathlib import Path
import platform
import shutil
import subprocess
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
HOSTS = {('Linux', 'x86_64'): ('linux', 'amd64'), ('Windows', 'AMD64'): ('windows', 'amd64'),
         ('Darwin', 'arm64'): ('darwin', 'arm64'), ('Darwin', 'x86_64'): ('darwin', 'amd64')}


def run(args, cwd=ROOT, env=None):
    subprocess.run(args, cwd=cwd, env=env, check=True)


def build(component, gui=False):
    goos, arch = HOSTS[(platform.system(), platform.machine())]
    suffix = '.exe' if goos == 'windows' else ''
    out = ROOT / 'dist' / component / (goos + '-' + arch)
    out.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, CGO_ENABLED='0', GOFLAGS='-mod=readonly')
    if component in ('engine', 'wrapper'):
        src = ROOT / 'compat/scan-engine'
        run(['cargo', '+1.88', 'build', '--locked', '--release', '-p', 'scan'], src)
        engine = src / 'target/release' / ('scan' + suffix)
        if component == 'engine':
            shutil.copy2(engine, out / ('scan' + suffix))
        else:
            # Intel macOS retains the original external-engine-only contract.
            if (goos, arch) == ('darwin', 'amd64'):
                raise SystemExit('Wrapper distributions support Linux x64, Windows x64, macOS arm64; use SCAN_ENGINE_PATH on Intel macOS.')
            payload = ROOT / 'compat/scan-fs/internal/engine/bin' / ('scan-' + goos + '-' + arch + suffix)
            shutil.copy2(engine, payload)
            run(['go', 'build', '-trimpath', '-o', str(out / ('scan-fs' + suffix)), './cmd/scan-fs'], ROOT / 'compat/scan-fs', env)
    elif component == 'catalog':
        for cmd in ('scan-cli', 'scan-git', 'scan-fs') + (('scan-gui',) if gui else ()):
            run(['go', 'build', '-trimpath', '-o', str(out / (cmd + suffix)), './cmd/' + cmd], ROOT / 'tools/project-catalog', dict(env, CGO_ENABLED='1' if cmd == 'scan-gui' else '0'))
    elif component == 'launchers':
        for alias, cmd in [('scan-projects', 'scan-git'), ('scan-export', 'scan-fs'), ('scanner', 'scan-gui')]:
            # Launchers are packaged separately and resolve only within their own component directory.
            p = out / (alias + ('.cmd' if goos == 'windows' else ''))
            p.write_text('@echo off\r\n"%~dp0' + cmd + '.exe" %*\r\n' if goos == 'windows' else '#!/bin/sh\nexec "$(dirname "$0")/' + cmd + '" "$@"\n')
            p.chmod(0o755)
    for license_name in ('LICENSE', 'LICENSE-MIT'):
        shutil.copy2(ROOT / license_name, out / license_name)
    shutil.copy2(ROOT / 'docs/consolidation/NOTICE.md', out / 'NOTICE.md')
    return out


def package(out):
    name = out.parent.name + '-' + out.name
    archive = ROOT / 'dist' / (name + ('.zip' if platform.system() == 'Windows' else '.tar.gz'))
    if archive.suffix == '.zip':
        with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED) as z:
            for p in sorted(out.iterdir()):
                z.write(p, name + '/' + p.name)
    else:
        with tarfile.open(archive, 'w:gz') as z:
            z.add(out, arcname=name)
    archive.with_name(archive.name + '.sha256').write_text(hashlib.sha256(archive.read_bytes()).hexdigest() + '  ' + archive.name + '\n')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('component', choices=['engine', 'wrapper', 'catalog', 'launchers'])
    parser.add_argument('--gui', action='store_true', help='Build Fyne on a native graphical host; does not qualify GUI operation')
    parser.add_argument('--archive', action='store_true')
    args = parser.parse_args()
    directory = build(args.component, args.gui)
    if args.archive:
        package(directory)
