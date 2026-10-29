#!/usr/bin/env python3
"""Native serial AX regression gate on a disposable synthetic Scanner process."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess
import time


def assess(samples):
    # Retain serial reads and all controls; do not mask latency with batched IPC.
    return bool(samples) and all(
        s['attribute_calls'] >= 20 and s['window_count'] >= 1
        and 0 <= s['median_ms'] < 10 and 0 <= s['snapshot_ms'] < 5000
        for s in samples
    ) and any(s['window_count'] >= 2 for s in samples)


def run(args):
    spec = importlib.util.spec_from_file_location('fixture', Path(__file__).with_name('platform-qualification.py'))
    fixture = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(fixture)
    root = args.evidence.resolve()
    if root.exists():
        raise ValueError('Use a new evidence directory; preserve earlier runs')
    env = fixture.prepare(root)
    p = subprocess.Popen([str(args.binary.resolve())], env=env,
                         stdout=(root / 'stdout.txt').open('w'), stderr=(root / 'stderr.txt').open('w'))
    report = dict(source=args.revision, binary_sha256=hashlib.sha256(args.binary.read_bytes()).hexdigest(),
                  passed=False, forced_cleanup=False, visual_qualification=False, release_acceptance=False)
    try:
        time.sleep(3)
        for name, action in [('main', ''), ('settings', 'cmd+comma')]:
            subprocess.run([str(args.helper.resolve()), str(p.pid), str(root / name), action, 'Scanner', '--ax-only'],
                           check=True, timeout=20, stdout=(root / (name + '.stdout.txt')).open('w'),
                           stderr=(root / (name + '.stderr.txt')).open('w'))
        samples = [json.loads(f.read_text()) for f in sorted(root.glob('*/accessibility-latency-*.json'))]
        report['samples'] = samples
        if not assess(samples):
            raise RuntimeError('serial AX median/snapshot or Settings coverage gate failed')
        start = time.monotonic()
        subprocess.run([str(args.helper.resolve()), str(p.pid), str(root / 'quit'), 'cmd+q'], check=True, timeout=8)
        if p.wait(timeout=max(.01, 8 - (time.monotonic() - start))) != 0:
            raise RuntimeError('native Quit failed')
        report.update(passed=True, quit_ms=(time.monotonic() - start)*1000)
    except Exception as error:
        report['error'] = str(error)
        raise
    finally:
        if p.poll() is None:
            p.kill()
            p.wait()
            report['forced_cleanup'] = True
        report['samples'] = [json.loads(f.read_text()) for f in sorted(root.glob('*/accessibility-latency-*.json'))]
        (root / 'report.json').write_text(json.dumps(report, indent=2) + '\n')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--helper', required=True, type=Path)
    parser.add_argument('--evidence', required=True, type=Path)
    parser.add_argument('--revision', required=True)
    args = parser.parse_args()
    if not re.fullmatch('[0-9a-f]{40}', args.revision):
        parser.error('revision must be a full source SHA')
    run(args)
