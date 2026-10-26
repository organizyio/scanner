# Mixed workload scale qualification

This follow-up exercises preferred Scanner's existing hashing and metadata paths.
It changes test tooling only. The production Rust/Go implementations, schemas,
facade signatures, dependency manifests/locks and consumer pins remain unchanged.

**PNG perceptual decoding remains unsupported in the default locked graph.**
The mixed probe verifies its explicit `phash:` error and absent perceptual value;
it does not count that as successful image decoding. [Issue #18](https://github.com/organizyio/scanner/issues/18)
tracks selecting and qualifying image decoder features separately. An embedding
consumer's feature graph may differ. Future codec enablement requires reviewing
the fixture expectation and rerunning affected qualification.

## Workload and use

Run from a clean committed checkout with Rust 1.88 installed:

```sh
python3 scripts/qualify-scale.py --output /private/evidence/new-mixed-run \
  --profiles mixed --files 512 2048 --repeats 3 --cancel-repeats 8
```

`--profiles small` remains the default, preserving the original 256-byte corpus.
Both profiles can be selected together when their file counts fit the mixed
limits. Mixed fixtures require at least 128 files. The harness rejects duplicate
profile/count arguments and any individual corpus above 512 MiB before building
or writing files; it never truncates a requested fixture. Use an unused private
output folder outside the checkout. Only owned temporary fixture directories
are deleted. Source identity and clean status are checked before and after runs.

Every 128 mixed file IDs contain one of each first-seven classes below, with the
remaining 121 IDs using the tiny payload. Roots/shards follow the original
Unicode multiroot recipe. Actual sizes, hashes and counts are in `fixture_specs`.

| Class | Synthetic payload | Checked result |
| --- | --- | --- |
| Empty | Zero bytes | Empty SHA-256, exact size and format record |
| Medium | 64 KiB binary | SHA-256 and byte count |
| Large | 1 MiB binary | SHA-256 and byte count |
| Huge | 8 MiB binary | SHA-256 and byte count |
| Image | Valid 1024×768 RGB PNG gradient | SHA-256/format and the known unsupported perceptual-decoder error |
| Document | DOCX with a 512 KiB uncompressed core description | ZIP inflation/XML parsing and exact creator, modifier and revision values |
| PDF | Valid synthetic 32-page document | Exact parsed page count |
| Tiny | 256-byte text | Original hash/size checks |

Files are actually written, not sparse-truncated, cloned or hard-linked. Binary
payloads repeat a fixed byte sequence. ZIP timestamps and attributes are fixed;
Python/zlib versions and resulting per-class hashes are recorded because
compressed bytes can vary across implementations. No personal documents,
external resources, `ffprobe`, audio or provider services are used.

Both walkers run fast callbacks, delayed callbacks, delayed NDJSON output,
cancellation and specific callback/output failures. Every partial/error case
then scans the entire corpus again in the same process. Record validation checks
class-specific path, size and digest, plus actual DOCX/PDF fields. A manual
negative control altered DOCX metadata and updated its digest; the probe still
rejected the field mismatch, proving metadata acceptance is not just hashing.

The probe retains a fixed eight-class descriptor/count table rather than all
records. Full completion checks totals, class counts and ID sum/XOR fingerprints;
these remain characterization checks, not a collision-free set proof. Partial
results report actual delivered bytes and class counts; the Python supervisor
cross-checks them against the generated specification and bounds. Known PNG error
counts are explicit. Unexpected record/extraction errors fail qualification.

## Measurements and boundaries

Report schema 2 adds profile-specific fixture specs, host/Python/zlib metadata,
class counts and explicit unsupported-image coverage. Existing small-profile
invocations remain valid. Elapsed/first-record/cancellation/recovery metrics and
POSIX per-child peak RSS retain the original scope. The capability preflight
occurs before scan timing; RSS includes process initialization and recovery.
Windows RSS remains explicitly unavailable, not a zero-memory result.

Cancellation is requested at an accepted-record boundary. A traversal-dependent
partial prefix may not include every costly class; delivered class counts expose
that scope and recovery checks the full corpus. These tests do not prove
preemption during active extraction or blocking I/O, nor worst-case latency or
process-memory bounds. Compressed XML/image/PDF parser work and filesystem cache
state affect measurements. No improvement claim is made against the small-file
baseline or another host.

CI uses 1,024 files for each profile, both walkers, one normal/failure repeat and
three cancellation repeats: 32 execution cases per OS, plus full recovery for
partial/error runs. Compile/CLI execution on a macOS arm64 CI runner does not
qualify interactive Apple Silicon/Retina hardware. Those gates stay pending;
runtime convergence, production adoption, publication and release acceptance
remain separate.

## Recorded Intel Mac run — 2026-10-26

[Normalized mixed results](mixed-scale-results-2026-10-26.json) retain exact source,
probe/fixture/harness hashes, host/tool versions and repeated metric ranges.
All 92 cases passed at 512 and 2,048 files, with three repeats per normal/failure
mode and eight cancellation repeats per fixture/walker. All 32 cancellation and
24 injected-failure cases passed their full in-process recovery scans. The source
remained clean and unchanged throughout qualification. Later result-documentation
commits do not change the measured probe, fixture recipe or runner.

The 2,048-file corpus includes sixteen 8 MiB binaries, sixteen 1 MiB binaries,
sixteen compressed DOCX core descriptions, sixteen 32-page PDFs and sixteen
PNG records with explicitly checked unsupported perceptual-decoder errors. A
passing mixed run means these declared outcomes were observed, not that every
image capability is available. Raw logs, complete measurements and negative
controls remain private. No throughput improvement, blocking-I/O deadline,
GUI qualification or release acceptance is inferred.
