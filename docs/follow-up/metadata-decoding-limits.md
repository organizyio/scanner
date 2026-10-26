# Preferred document and EXIF metadata budgets

These changes apply to preferred Scanner's PDF, DOCX and EXIF readers. Public
facade signatures, schemas, field names/types and compatibility implementations
remain unchanged. Existing best-effort `Option` results remain: malformed,
incomplete or over-budget extraction returns unknown/absent metadata. A file row
and independently requested hashing still proceed. The API does not expose a
new diagnostic distinguishing missing metadata from a parser limit.

| Resource | Limit and enforcement |
| --- | --- |
| PDF/DOCX encoded input | 64 MiB, checked against the opened file and an actual read capped at limit + one byte |
| EXIF container encoded input | 32 MiB, with the same actual-read check |
| DOCX parser reads | 8 MiB cumulatively across seeks, inside the bounded snapshot |
| DOCX archive entries | 4,096, checked after ZIP directory parsing |
| DOCX core XML | 2 MiB, checked against declared size and actual expanded bytes |
| XML complexity | 50,000 nodes; DTD disabled |
| PDF eager object/xref stream decompression | 8 MiB per stream through pinned lopdf load options |
| PDF loaded objects | 100,000, checked after loading |
| PDF page-tree traversal | At most 100,000 discovered/pending references and depth 256; iterative traversal rejects cycles, duplicates, missing children and invalid node types |
| EXIF parser reads/raw TIFF data | 2 MiB cumulatively across reads/seeks; raw-data length also checked |
| EXIF fields | 4,096, checked before lazy field materialization |
| EXIF exported values | 262,144 aggregate scalar/byte units before formatting or JSON conversion; ASCII includes bytes and parts, rationals count two units |

Input is read once into bounded bytes. This is not an atomic filesystem snapshot
if another process modifies the file while that initial read is active. ZIP
members are read by name, never extracted to disk. Whole EXIF extraction is
declined above its limits rather than truncating extras. Valid Unicode strings,
revision strings such as `007`, large unsigned values, numeric arrays and
zero-denominator rational nulls retain their existing representation. Camera
formatting and GPS conversion are unchanged.

PDF page count now requires a complete valid reachable page tree. It counts Page
nodes rather than trusting declared Count, and declines a partial result from a
cyclic, duplicated or incomplete tree. A document containing an unrelated bad
object stream can still have readable page metadata. This is not a promise to
reject every oversized stream anywhere in a document, or new encrypted-PDF
support.

## Limits of the limits

These are not process-RSS, aggregate decompression, CPU or elapsed-time ceilings.
The snapshot, parser indexes, materialized values and output may coexist. ZIP
entry and PDF object limits are checked after library allocations; PDF's
per-stream budget is not an aggregate allocation budget. EXIF values are
materialized before their aggregate export check. Existing parser allocation
behavior remains relevant. Blocking I/O, active extraction and user callbacks
are not preempted. Other preferred metadata extractors and the compatibility
engine/wrapper retain their behavior. Untrusted-input sandboxing remains a
separate concern.

## Validation

[Deterministic synthetic fixtures](../../crates/metadata/tests/fixtures/manifest.json)
cover valid and declined DOCX, PDF and TIFF controls. The standard-library
[generator](../../scripts/generate-metadata-fixtures.py) records exact hashes.
Unit tests add inclusive limits, stored/deflated XML, forged advertised sizes,
node/entry limits, cyclic/duplicate/missing page references, compressed PDF
object streams with a working unbounded-loader control, EXIF field/value limits,
forged counts, truncation and subsequent recovery. Sparse encoded-overflow files
exercise all readers without checking large fixtures into Git.

[Public facade and actual CLI tests](../../scanfs/tests/metadata_limits.rs)
require six complete rows, unchanged SHA-256 values and JSON types, absent
metadata for declined controls and repeated recovery. These tests do not assert
a global resource ceiling or claim exhaustive fuzzing. Run pinned Rust checks,
fixture regeneration, consolidation verification and component/consumer gates.
[AccessKit qualification](accesskit-metadata-qualification.md) records this
follow-up's native scope separately. Production pins, runtime convergence,
publication and release acceptance remain unchanged.
