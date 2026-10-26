# Preferred PNG decoding limits

This follow-up preserves the `phash_u64`, Scanner facade and CLI signatures,
`PhashError` variants, record schema and 8×8 gradient hash. It replaces the old
image wrapper's unlimited PNG reader with pinned `png 0.18.1`, converting decoded
samples into the same `image 0.23.14` types. Other formats retain the existing
fallback; no additional codec, compatibility engine or Go path is enabled.

| Resource | Limit and enforcement |
| --- | --- |
| Encoded PNG input | 32 MiB. Reject oversized file metadata, then cap the actual read at limit + one byte and reject overflow, including growth after metadata inspection |
| Width and height | Each at most 16,384 pixels. Inspect the PNG header before reading frame metadata or allocating the output pixel buffer |
| Decoded output | At most 64 MiB of transformed samples, checked before caller-owned pixel allocation, with checked size arithmetic from the PNG reader |
| Decoder intermediate allocations | Pass a 64 MiB budget to the PNG library; this library limit is best effort and excludes caller-owned buffers |

These fixed defaults apply only to preferred PNG perceptual extraction. Images
above a limit now produce a resource-limit error instead of a perceptual value;
they are not silently downsampled to a different hash. Limit failures retain
`PhashError::Image(ImageError::Limits(...))`. CLI rows retain a `phash:` error and
absent `meta.phash`; traversal and subsequent files/scans continue normally.
Dimension errors display `Image is too large`; encoded/decoded budget errors
display `Insufficient memory` through the existing image error type.

Decode the same bounded encoded byte buffer, without reopening the path between
header checks and pixel conversion. This does not make the initial filesystem
read an atomic snapshot if another process changes the file. PNG text and ICC metadata are ignored by
this sample-based hash. EXPAND retains bit depth; 16-bit network-order samples
are converted to the existing native u16 image buffers. Existing RGB/RGBA, gray,
alpha, palette/transparency, 16-bit and Adam7 goldens remain required; new 16-bit
gray-alpha/RGBA fixtures cover the remaining conversion arms. The original nine
positive image files retain their exact bytes. Required values include
the mixed PNG golden `13089050091743192493`. Alpha remains ignored by luminance
hashing. Animation playback and rendered color-management fidelity are not
additional support claims.

These are not a 64 MiB process-RSS guarantee. Encoded bytes, decoder allocations,
output samples, temporary 16-bit conversion buffers and hashing/resizing work
can coexist, and other metadata extractors have their own behavior. Limits do
not preempt blocking filesystem I/O or active decoding, establish a CPU deadline,
or provide an untrusted-image sandbox. Existing queue/cancellation contracts
remain unchanged.

## Qualification and provenance

[Header-only controls](../../crates/phash/tests/limits/manifest.json) contain valid
CRCs and deliberately absent pixel payloads. Oversized dimensions and decoded
output must return limit errors before reaching that payload. The
[generator](../../scripts/generate-png-limit-fixtures.py) is standard-library-only;
regeneration tests retain exact hashes. Private helper tests cover inclusive
budget edges and an infinite reader that grows beyond the reported size using a
small test cap. Deterministic truncation/byte-mutation controls check for panics;
they are regression samples, not exhaustive fuzzing.

Public hash tests, public Scanner facade and actual CLI tests cover oversized
encoded files, dimensions/output budgets, truthful per-record errors, valid
records alongside errors and repeated scan recovery. Original valid-image,
corrupt/truncated and unsupported-JPEG regressions remain. `make check`, pinned
Rust 1.88 tests, unchanged-policy advisory/license checks, Python/fixture tests,
consolidation verification and three-platform scale/component CI are required.

[Dependency review](maintenance-png-review-2026-10-26.md) records the PNG reader
and narrow inout lock update. [PR #20's initial decoder evidence](png-perceptual-hashing.md)
and [its consumer completion](https://github.com/organizyio/scanner/issues/18#issuecomment-6407572436)
remain historical. The qualification register records that published baseline
separately from this follow-up. This PR's checks and follow-up consumer result
record the new candidate; the consumer resolver requires a reviewed main commit,
so its final exact-candidate run follows merge. Temporary worker patches/tests,
lock changes and embedded binaries must be restored; production pins stay fixed.

The catalog/native/toolkit tree is unchanged. The original Apple Silicon/Retina
handoff and hardware gates remain pending; CLI CI is not interactive hardware
evidence. Runtime convergence, publication, production adoption and release
acceptance are excluded.
