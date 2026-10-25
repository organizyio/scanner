# Preferred Scanner scale qualification

This harness adds repeatable synthetic execution measurements without changing
production scanner code, facade signatures, schemas, compatibility components or
consumer pins. It does not exercise the Go catalog/GUI or converge runtimes.

From a clean committed checkout, with Rust 1.88 installed:

```sh
python3 scripts/qualify-scale.py --output /private/evidence/new-scale-run \
  --files 4096 32768 --repeats 3 --cancel-repeats 8 --delay-us 25
```

The harness builds its Rust example from that checkout with pinned Rust and
`--locked`, using a fresh private target directory. Results record source SHA,
probe hash, tool/host metadata and fixture parameters. Output must be a new
folder outside the checkout. No production or personal folders are scanned.
Only the harness's temporary synthetic fixture directories are removed.

Each fixture has two disjoint roots (including Unicode), 32 deterministic shards
and sequential IDs with identical 256-byte payloads. Both standard and full
walkers execute fast callbacks, delayed callbacks, delayed NDJSON output,
requested cancellation, callback failure and broken output. Cancellation repeats
independently of throughput cases. Every partial/error case then performs a
complete scan in the same process to verify recovery.

The constant-size consumer validates each record's expected root/path, size and
SHA-256, delivered byte/count totals and commutative ID sum/XOR fingerprints.
Those fingerprints detect common omissions/duplication; they are not a
collision-free set proof. The output sink buffers at most one bounded record.
Failure cases require the specific injected error, not any nonzero result.
An external deadline kills/reaps only the owned probe and fails the run rather
than reporting a timeout as cancellation success.

`report.json` records elapsed time, time to first accepted record, requested
cancellation-to-return latency and post-stop recovery timing for every run.
Per-child `wait4` supplies high-water RSS on Linux/macOS, including process/runtime
and recovery. Windows has no corresponding stdlib measurement and reports null
with an explicit reason; execution cases still run there. Do not compare these
measurements with unrelated debug builds or infer a hard RSS bound from them.

The existing deterministic engine tests separately prove the 256-record queue,
caller-thread callbacks, early delivery and joined failure/cancellation cleanup.
Scale runs exercise sustained slow consumption; they cannot observe queue
occupancy directly. Active extraction, blocked callbacks and OS I/O remain
non-preemptible. Timings vary with scheduler granularity, warm caches, hardware
and filesystem. Repeated observations establish neither an improvement claim nor
a worst-case cancellation guarantee.

CI runs the same source-built probe on Linux, Windows and macOS with a smaller
1,024-file fixture, one throughput/failure repeat and three cancellation repeats.
Only normalized reports are uploaded as CI evidence, not release artifacts.
This qualifies synthetic preferred-Scanner execution only. Apple Silicon/Retina
GUI feedback remains pending. Production adoption, runtime convergence and release
acceptance remain separate.
