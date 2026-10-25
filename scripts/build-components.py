#!/usr/bin/env python3
"""Build isolated migration components from local reviewed source; never download engines."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
HOSTS = {('Linux', 'x86_64'): ('linux', 'amd64'), ('Windows', 'AMD64'): ('windows', 'amd64'),
         ('Darwin', 'arm64'): ('darwin', 'arm64'), ('Darwin', 'x86_64'): ('darwin', 'amd64')}


def run(args, cwd=ROOT, env=None):
    subprocess.run(args, cwd=cwd, env=env, check=True)


def copy_notice(source, destination):
    destination.parent.mkdir(parents=True, exist_ok=True)
    if destination.exists():
        destination.chmod(0o644)
    shutil.copyfile(source, destination)
    destination.chmod(0o644)


def dependency_notices(out, component, gui=False):
    """Retain license/notice texts from the exact public dependency sources."""
    packages = []
    if component in ('engine', 'wrapper'):
        metadata = json.loads(subprocess.check_output([
            'cargo', '+1.88', 'metadata', '--locked', '--format-version', '1',
            '--manifest-path', str(ROOT / 'compat/scan-engine/Cargo.toml')], cwd=ROOT))
        host = subprocess.check_output(['rustc', '+1.88', '-vV'], text=True, encoding='utf-8').split('host: ')[1].splitlines()[0]
        tree = subprocess.check_output(['cargo', '+1.88', 'tree', '--locked', '--prefix', 'none', '--edges', 'normal',
                                        '--target', host, '-p', 'scan', '--format', '{p}'], cwd=ROOT / 'compat/scan-engine', text=True, encoding='utf-8')
        used = {tuple(line.split()[:2]) for line in tree.splitlines()}
        packages += [(p['name'] + '-' + p['version'], Path(p['manifest_path']).parent)
                     for p in metadata['packages'] if p['source'] is not None and (p['name'], 'v' + p['version']) in used]
    if component == 'catalog' and gui:
        native = ROOT / 'tools/project-catalog/native-a11y'
        metadata = json.loads(subprocess.check_output([
            'cargo', '+1.88', 'metadata', '--locked', '--format-version', '1',
            '--manifest-path', str(native / 'Cargo.toml')], cwd=ROOT))
        host = ('x86_64-pc-windows-gnu' if platform.system() == 'Windows' else
                subprocess.check_output(['rustc', '+1.88', '-vV'], text=True, encoding='utf-8').split('host: ')[1].splitlines()[0])
        tree = subprocess.check_output([
            'cargo', '+1.88', 'tree', '--locked', '--prefix', 'none', '--edges', 'normal',
            '--target', host, '-p', 'scanner-native-a11y', '--format', '{p}'],
            cwd=native, text=True, encoding='utf-8')
        used = {tuple(line.split()[:2]) for line in tree.splitlines()}
        packages += [(p['name'] + '-' + p['version'], Path(p['manifest_path']).parent)
                     for p in metadata['packages'] if
                     (p['source'] is not None or Path(p['manifest_path']).is_relative_to(native / 'vendor')) and
                     (p['name'], 'v' + p['version']) in used]
    if component in ('wrapper', 'catalog'):
        module = ROOT / ('compat/scan-fs' if component == 'wrapper' else 'tools/project-catalog')
        commands = ['./cmd/scan-fs'] if component == 'wrapper' else ['./cmd/scan-cli', './cmd/scan-git', './cmd/scan-fs']
        if gui and component == 'catalog':
            commands.append('./cmd/scan-gui')
        raw = subprocess.check_output(['go', 'list', '-mod=readonly', '-deps', '-json', *commands], cwd=module,
                                      env=dict(os.environ, CGO_ENABLED='1' if gui else '0'), text=True, encoding='utf-8')
        decoder = json.JSONDecoder()
        modules = {}
        while raw.strip():
            entry, end = decoder.raw_decode(raw.lstrip())
            raw = raw.lstrip()[end:]
            mod = entry.get('Module', {})
            if not mod.get('Main') and mod.get('Dir'):
                modules[mod['Path']] = mod
        packages += [(m['Path'].replace('/', '_') + '-' + m['Version'], Path(m['Dir'])) for m in modules.values()]
        goroot = Path(subprocess.check_output(['go', 'env', 'GOROOT'], text=True, encoding='utf-8').strip())
        notices = out / 'third-party/golang'
        notices.mkdir(parents=True, exist_ok=True)
        go_license = goroot / 'LICENSE'
        if not go_license.exists():
            go_license = goroot.parent / 'LICENSE'  # Homebrew separates licenses from libexec.
        copy_notice(go_license, notices / 'LICENSE')
    if component in ('engine', 'wrapper') or (component == 'catalog' and gui):
        sysroot = Path(subprocess.check_output(['rustc', '+1.88', '--print', 'sysroot'], text=True, encoding='utf-8').strip())
        rust_notices = sysroot / 'share/doc/rust'
        shutil.copytree(rust_notices / 'licenses', out / 'third-party/rust/licenses', dirs_exist_ok=True)
        shutil.copy2(rust_notices / 'COPYRIGHT-library.html', out / 'third-party/rust/COPYRIGHT-library.html')
    index = []
    for name, source in packages:
        retained = []
        for path in sorted(source.rglob('*')):
            if not path.is_file() or path.is_symlink():
                continue
            if not (path.name.upper().startswith(('LICENSE', 'LICENCE', 'COPYING', 'NOTICE', 'COPYRIGHT', 'AUTHORS')) or
                    any(part.upper() in ('LICENSES', 'LICENCES') for part in path.relative_to(source).parts[:-1])):
                continue
            if path.suffix.lower() in ('.go', '.rs', '.png', '.jpg'):
                continue
            # Only copy text notices from dependencies, never executables or caches.
            path.read_text(encoding='utf-8')
            relative = path.relative_to(source)
            target = out / 'third-party' / name / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            copy_notice(path, target)
            retained.append(str(relative))
        fallback = ROOT / 'licenses/upstream' / name
        if not retained and fallback.exists():
            for path in sorted(fallback.iterdir()):
                target = out / 'third-party' / name / path.name
                target.parent.mkdir(parents=True, exist_ok=True)
                copy_notice(path, target)
                retained.append(path.name)
        if not retained:
            raise RuntimeError('Required upstream notice missing for ' + name)
        if component == 'catalog' and gui and source == ROOT / 'tools/project-catalog/native-a11y/vendor/accesskit_unix':
            provenance = ROOT / 'tools/project-catalog/native-a11y/vendor/provenance.json'
            shutil.copy2(provenance, out / 'third-party' / name / 'SCANNER-UPSTREAM-PATCH.json')
            retained.append('SCANNER-UPSTREAM-PATCH.json')
        index.append({'package': name, 'license_notice_files': retained})
    if packages:
        (out / 'third-party').mkdir(exist_ok=True)
        (out / 'third-party/index.json').write_text(json.dumps(index, indent=2) + '\n')
        shutil.copy2(ROOT / 'licenses/upstream/manifest.json', out / 'third-party/UPSTREAM-NOTICES.json')


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
        if gui:
            run([sys.executable, str(ROOT / 'scripts/build-native-a11y.py')])
        for cmd in ('scan-cli', 'scan-git', 'scan-fs') + (('scan-gui',) if gui else ()):
            flags = []
            if cmd == 'scan-gui':
                # Go does not track external Rust static archives in its cache.
                # Include exact archive bytes in the link action's cache key.
                native = ROOT / 'tools/project-catalog/native-a11y/target/release/libscanner_native_a11y.a'
                flags = ['-ldflags=-buildid=scanner-native-a11y-' + hashlib.sha256(native.read_bytes()).hexdigest()]
            run(['go', 'build', '-trimpath', *flags, '-o', str(out / (cmd + suffix)), './cmd/' + cmd], ROOT / 'tools/project-catalog', dict(env, CGO_ENABLED='1' if cmd == 'scan-gui' else '0'))
    elif component == 'launchers':
        for alias, cmd in [('scan-projects', 'scan-git'), ('scan-export', 'scan-fs'), ('scanner', 'scan-gui')]:
            # Launchers are packaged separately and resolve only within their own component directory.
            p = out / (alias + ('.cmd' if goos == 'windows' else ''))
            p.write_text('@echo off\r\n"%~dp0' + cmd + '.exe" %*\r\n' if goos == 'windows' else '#!/bin/sh\nexec "$(dirname "$0")/' + cmd + '" "$@"\n')
            p.chmod(0o755)
    for license_name in ('LICENSE', 'LICENSE-MIT'):
        shutil.copy2(ROOT / license_name, out / license_name)
    shutil.copy2(ROOT / 'docs/consolidation/NOTICE.md', out / 'NOTICE.md')
    dependency_notices(out, component, gui)
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True, encoding='utf-8').strip()
    (out / 'BUILD.json').write_text(json.dumps({'component': component, 'scanner_commit': commit,
        'platform': goos + '-' + arch, 'gui_compiled': gui, 'release_acceptance': False}, indent=2) + '\n')
    return out


def package(out):
    name = out.parent.name + '-' + out.name
    archive = ROOT / 'dist' / (name + ('.zip' if platform.system() == 'Windows' else '.tar.gz'))
    if archive.suffix == '.zip':
        with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED, strict_timestamps=False) as z:
            for p in sorted(out.rglob('*')):
                if p.is_file():
                    z.write(p, name + '/' + str(p.relative_to(out)))
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
