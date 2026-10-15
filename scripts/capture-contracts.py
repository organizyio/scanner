#!/usr/bin/env python3
"""Capture synthetic CLI contracts and measured execution; never inspect user roots."""
import argparse
import json
import os
from pathlib import Path
import platform
import re
import signal
import subprocess
import tempfile
import threading
import time


def normalize(value, root, key=''):
    if isinstance(value, dict):
        return {str(k).replace(str(root), '<fixture>'): normalize(v, root, k) for k, v in sorted(value.items())}
    if isinstance(value, list):
        values = [normalize(v, root) for v in value]
        return sorted(values, key=lambda v: json.dumps(v, sort_keys=True))
    if key in ('inode', 'device_id', 'depth'):
        return 0
    if key.endswith('_at') and isinstance(value, str) and value:
        return '<timestamp>'
    return value.replace(str(root), '<fixture>') if isinstance(value, str) else value


def read_process(command, env, stderr, cancel=False, timeout=60):
    """Deadline covers pipe reads as well as wait; kill the synthetic process group."""
    proc = subprocess.Popen(command, env=env, stdout=subprocess.PIPE, stderr=stderr,
                            start_new_session=os.name != 'nt')
    expired = threading.Event()
    def terminate():
        expired.set()
        if os.name == 'nt':
            subprocess.run(['taskkill', '/PID', str(proc.pid), '/T', '/F'], capture_output=True)
        else:
            try:
                os.killpg(proc.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
    timer = threading.Timer(timeout, terminate)
    timer.start()
    start = time.perf_counter()
    try:
        first = proc.stdout.readline()
        first_time = time.perf_counter() - start
        sent = time.perf_counter()
        if cancel and proc.poll() is None:
            proc.send_signal(signal.SIGINT)
        remaining = proc.stdout.read()
        exit_code = proc.wait()
        if expired.is_set():
            raise TimeoutError('synthetic scanner exceeded capture deadline')
        return first, remaining, exit_code, time.perf_counter() - start, first_time, time.perf_counter() - sent
    finally:
        timer.cancel()
        timer.join()
        if proc.poll() is None:
            terminate()
            proc.wait()
        proc.stdout.close()


def capture(scanfs, catalog):
    env = dict(os.environ, SCAN_FS_SOURCE='synthetic-contract-fixture')
    with tempfile.TemporaryDirectory(prefix='scanner-contract-') as directory:
        root = Path(directory)
        fixture = root / 'input'
        fixture.mkdir()
        (fixture / 'hello.txt').write_text('abc')
        (fixture / 'node').mkdir()
        (fixture / 'node/package.json').write_text('{}')
        preferred = subprocess.run([scanfs, '--root', str(fixture), '--profile', 'fast', '--format', 'json'], env=env, capture_output=True, text=True, check=True, timeout=60)
        rows = [json.loads(line) for line in preferred.stdout.splitlines()]
        deep = root / 'deep.json'
        catalog_out = root / 'catalog.json'
        subprocess.run([catalog, 'fs', '--root', str(fixture), '--output', str(deep), '--no-hash'], capture_output=True, check=True, timeout=60)
        subprocess.run([catalog, 'catalog', '--root', str(fixture), '--output', str(catalog_out)], capture_output=True, check=True, timeout=60)
        cases = {}
        for name, command in [('preferred_help', [scanfs, '--help']), ('preferred_invalid', [scanfs, '--invalid-flag']), ('catalog_help', [catalog, 'catalog', '--help']), ('catalog_invalid', [catalog, 'catalog', '--invalid-flag']), ('deep_help', [catalog, 'fs', '--help'])]:
            result = subprocess.run(command, capture_output=True, text=True, timeout=60)
            cases[name] = {'exit': result.returncode, 'stdout': result.stdout, 'stderr': result.stderr}
        contracts = normalize({'preferred': rows, 'catalog': json.loads(catalog_out.read_text()), 'deep': json.loads(deep.read_text()), 'cli': cases}, root)
        # Exactly 4096 files / 16 MiB of synthetic payload, always outside source trees.
        bench = root / 'benchmark'
        bench.mkdir()
        for i in range(4096):
            (bench / f'{i:05}.txt').write_bytes(b'x' * 4096)
        measurements = []
        for cancel in (False, True):
            if cancel and os.name == 'nt':
                measurements.append({'interrupted': True, 'status': 'not_measured', 'reason': 'Use the native Windows console qualification runner for control-event delivery.'})
                continue
            command = [scanfs, '--root', str(bench), '--profile', 'fast', '--format', 'json']
            wrapper = []
            if not cancel and platform.system() == 'Darwin':
                wrapper = ['/usr/bin/time', '-l']
            elif not cancel and platform.system() == 'Linux' and Path('/usr/bin/time').exists():
                wrapper = ['/usr/bin/time', '-v']
            with tempfile.TemporaryFile() as stderr:
                first, remaining, exit_code, elapsed, first_time, cancellation_time = read_process(wrapper + command, env, stderr, cancel)
                stderr.seek(0)
                text = stderr.read().decode('utf-8', errors='replace')
                match = re.search(r'(\d+)\s+maximum resident set size', text)
                linux_match = re.search(r'Maximum resident set size \(kbytes\):\s*(\d+)', text)
                memory = int(match[1]) if match else (int(linux_match[1]) * 1024 if linux_match else None)
                measurements.append({'interrupted': cancel, 'exit': exit_code, 'elapsed_seconds': elapsed if not cancel else None, 'time_to_first_record_seconds': first_time if not cancel else None, 'cancellation_seconds': cancellation_time if cancel else None, 'peak_rss_bytes': memory if not cancel else None, 'records': bool(first) + len(remaining.splitlines())})
        return contracts, {'platform': platform.system() + '-' + platform.machine(), 'fixture_files': 4096, 'fixture_bytes': 16777216, 'measurements': measurements, 'notes': 'Local debug build; no cross-host performance or release claim. Cancellation starts after first record; baseline traversal may already be complete.'}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--scanfs', required=True)
    parser.add_argument('--catalog', required=True)
    parser.add_argument('--contracts', required=True)
    parser.add_argument('--measurements', required=True)
    args = parser.parse_args()
    contracts, measurements = capture(str(Path(args.scanfs).resolve()), str(Path(args.catalog).resolve()))
    for path, value in [(args.contracts, contracts), (args.measurements, measurements)]:
        Path(path).write_text(json.dumps(value, indent=2, sort_keys=True) + '\n')
