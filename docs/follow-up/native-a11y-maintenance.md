# Native accessibility dependency maintenance

Reviewed 2026-10-24. The isolated native bridge remains source-built with Rust
1.88; production Scanner consumers and preferred crate pins are unchanged.

## AccessKit Unix backport

The [upstream cache fix](https://github.com/AccessKit/accesskit/pull/819) is merged
as `bffadacde348b0b73d3d7201bfb0b7f69a54b0bf`. The latest published Unix release
verified through the [upstream release list](https://github.com/AccessKit/accesskit/releases)
is still `accesskit_unix-v0.24.0`. That original release predates the fix.
Retain the reviewed backport and its immutable archive/file provenance.
A merged upstream fix alone does not make removing the patch safe.

Before retiring the vendored patch:

- Verify a published crate actually contains the cache signal fix and identify its exact archive checksum and MSRV.
- Review the dependency/notice delta and qualify it in the isolated native workspace. Keep preferred Scanner and compatibility pins separate.
- Require native cache wire tests, the negative control, C lifecycle regressions, and GUI/control tests. Compilation alone cannot establish operation.
- Replay available native hosts and record missing hosts honestly. Preserve historical import evidence and upstream license grants.
- Remove the patch only in a focused reviewed PR with a tested rollback. Never regenerate immutable import hashes to satisfy a check.

## Text scope

The bridge maps bounded single-line, non-password entry selection in Unicode
scalar indexes. Native range requests are checked against the owning text run,
current contents, enabled state and current modal tree. Selection does not emit
value-edit callbacks or alter profile/checkpoint formats.

Fyne's public API exposes selected text but not its anchor. If repeated text
makes the anchor ambiguous, omit the observed range rather than invent its
direction. Multiline/CRLF fields, password fields and entries over 4096 Unicode
scalars do not advertise range actions. These remain explicit follow-ups for a
public upstream range API and dedicated mapping tests. Native protocol evidence
is separate from spoken-reader usability and release acceptance.
