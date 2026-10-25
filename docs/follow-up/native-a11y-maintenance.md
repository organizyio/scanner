# Native accessibility dependency maintenance

Reviewed 2026-10-24. The isolated native bridge remains source-built with Rust
1.88; production Scanner consumer pins are unchanged. AccessKit pins and the
reviewed backport remain unchanged; separate metadata ZIP security maintenance
is recorded in [its review](zip-security.md).

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

## Linux type-graph maintenance

The [native informational advisory review](native-advisories.md) tracks the
zbus 5 dependencies retained by published AccessKit/AT-SPI. The isolated policy
is version-bounded and expires before 2026-11-24; required CI rejects drift or
expiry. This disposition does not authorize vulnerability exceptions, an
unreviewed zbus 6 port, or release acceptance.

## Fyne graphics queue repair

The [reviewed Fyne v2.7.3 adaptation](../../tools/project-catalog/third_party/README.md)
addresses the subsequently reproduced Windows shutdown race. It preserves the
version and public interfaces through a local source replacement. Remove that
adaptation only after verifying both atomic shutdown submission and complete
drain acknowledgements in a published release, then replay the native gates.
The recorded Windows scope does not qualify other hosts or release acceptance.

[The subsequent Linux and available Mac replay](linux-macos-replay.md) extends
operation evidence for the unchanged toolkit adaptation. Its default Linux
presentation failure, unavailable hardware and broader reader gates remain
explicit. It does not retire either reviewed dependency adaptation.

[The live Linux desktop follow-up](linux-default-rendering.md) subsequently
qualifies default-mode presentation with unchanged toolkit/bridge source. It
adds readiness and actual foreground guards to qualification; it does not retire
reviewed dependency adaptations or approve releases.
