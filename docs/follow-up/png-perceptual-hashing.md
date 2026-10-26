# PNG perceptual hashing — issue #18

The preferred default build explicitly enables **PNG only** on the existing
`img_hash 3.2.0` / `image 0.23.14` graph. The direct image requirement is exact,
with default features disabled. JPEG, GIF, WebP, TIFF and HEIC are not qualified
or enabled by this change. Downstream feature unification can add codecs; that
is not a promise made by this default build. Compatibility components, facade
signatures, record schemas, package versions and production consumer pins remain
unchanged. No image parser or hashing implementation is replaced.

## Contract and fixtures

The existing 8×8 gradient hash, first eight bytes and big-endian `u64` conversion
are retained. [Synthetic fixture generator](../../scripts/generate-png-fixtures.py)
uses only Python's standard library and records source-file digests in
[its manifest](../../crates/phash/tests/fixtures/manifest.json). The 9×8 pixel
fixtures need no resize: alternating ascending/descending grayscale rows have
eight true/false comparisons, hence `0xff00ff00ff00ff00`; a constant image hashes
to zero. RGB/RGBA, gray/gray-alpha, indexed with transparency, gray/RGB 16-bit and
Adam7 encode that same pixel oracle. This is representative coverage, not an
exhaustive claim about all PNG encodings, color management or hostile inputs.

`cargo test --locked -p scanner-phash --test png` checks the public hash function.
`cargo test --locked -p scanfs --test png` checks the public Scanner facade and
actual CLI, Unicode/uppercase paths, opt-in behavior, SHA-256/schema retention,
and corrupt, truncated and unsupported-file error records. Successful completion
can contain per-file errors; those records must have no perceptual value.
Tests do not enable codecs through dev dependencies. The ordinary source-built
CLI mixed probe also requires successful decoding with a fixed golden value.

The mixed PNG's golden `13089050091743192493` is additionally characterized from
raw synthetic pixels without PNG decoding. It is fixed in the probe, never
calculated from the decoder under test at runtime. All class digests, DOCX/PDF
fields and recovery checks remain required. Original
[mixed results](mixed-scale-results-2026-10-26.json) retain the earlier unsupported
outcome; new measurements cannot reinterpret or replace them.

## Dependency review

[New locked registry nodes and checksums](png-dependencies.json) are `png 0.16.8`,
`adler32 1.2.0`, `deflate 0.8.6`, `miniz_oxide 0.3.7` and `bitflags 1.3.2`.
Their declared licenses are respectively MIT OR Apache-2.0, Zlib, MIT/Apache-2.0,
MIT, and MIT/Apache-2.0; packaged upstream notices were reviewed. Existing `image`
is MIT. These older crates do not declare MSRVs; actual pinned Rust 1.88 builds
and cross-platform tests are required instead of inferring compatibility.
The lock retains exact registry checksums. No private code/history is imported.

The unchanged cargo-deny policy passes this graph with no new exception. This
means no matching advisory at review, not proof of parser safety. Existing
informational notices and unrelated lock maintenance remain separately recorded.
The narrow feature selection avoids silently enabling image's other codecs or
changing gradient/resizing behavior. Updating this older image stack is a
separate, behavior-sensitive dependency review. No decoder allocation bound,
worst-case latency, active-extraction preemption or untrusted-image sandbox is
established here. Existing extraction/cancellation limits still apply.

## Evidence and gates

Local and exact-head CI evidence and the subsequent reviewed-main consumer run
are recorded with the results of this change. Required CI exercises facade/CLI
fixtures and source-built mixed probes on Linux, Windows and macOS; compatibility
integration and the catalog matrix remain required. Consumer qualification uses
an explicit reviewed-main candidate, temporary crate patches and unchanged-pin
rollback. It does not adopt production dependencies.

Apple Silicon interactive and actual Retina display transitions remain pending
external feedback. CLI execution on an arm64 runner does not satisfy those gates.
Runtime convergence, GUI support expansion, publication and release acceptance
are excluded.
