#!/usr/bin/env python3
"""Fail closed on drift or expiry of reviewed native-only informational notices."""
import copy
from datetime import date
from pathlib import Path
import tomllib

ROOT = Path(__file__).resolve().parents[1]
REVIEW_BEFORE = date(2026, 11, 24)
EXPECTED = {
    'RUSTSEC-2026-0370': ('zvariant', '5.15.0'),
    'RUSTSEC-2026-0371': ('zbus_names', '4.3.4'),
    'RUSTSEC-2026-0372': ('zvariant_derive', '5.15.0'),
}


def verify(root_policy, native_policy, lock, today):
    if today >= REVIEW_BEFORE:
        raise ValueError('Native informational advisory review expired; re-review upstream before merge')
    expected = copy.deepcopy(root_policy)
    exceptions = [e for e in native_policy['advisories']['ignore'] if e['id'] in EXPECTED]
    if len(exceptions) != len(EXPECTED) or {e['id'] for e in exceptions} != set(EXPECTED):
        raise ValueError('Native informational dispositions must match exactly the reviewed three IDs')
    for entry in exceptions:
        if '2026-11-24' not in entry.get('reason', '') or 'informational unmaintained' not in entry.get('reason', ''):
            raise ValueError('Missing native disposition scope or review date')
    expected['advisories']['ignore'] = exceptions + expected['advisories']['ignore']
    if native_policy != expected:
        raise ValueError('Native policy must otherwise equal root policy; no other relaxation permitted')
    for name, version in EXPECTED.values():
        versions = [p['version'] for p in lock['package'] if p['name'] == name]
        if versions != [version]:
            raise ValueError('Native dependency version changed; re-review disposition: ' + name)


def main():
    native = ROOT / 'tools/project-catalog/native-a11y'
    load = lambda p: tomllib.loads(p.read_text())
    verify(load(ROOT / 'deny.toml'), load(native / 'deny.toml'), load(native / 'Cargo.lock'), date.today())
    print('Native-only informational dispositions match reviewed versions and remain within review window')


if __name__ == '__main__':
    main()
