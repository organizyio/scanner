# Published AccessKit and metadata hardening qualification

Reviewed 2026-10-26 against Scanner main
`960ed1faf7f5e158f5345ce36e24108d18c619a6` (PR #21). This follow-up adopts a
reviewed published accessibility dependency graph and adds
[preferred metadata budgets](metadata-decoding-limits.md). It does not adopt
production consumer pins, converge runtimes or approve a release.

## Dependency and provenance review

The isolated native graph uses registry `accesskit 0.25.2`,
`accesskit_consumer 0.39.2`, `accesskit_atspi_common 0.21.1` and
`accesskit_unix 0.24.1`. macOS adapter `0.27.1` and Windows adapter `0.35.1`
remain pinned. The four reviewed package upgrades are the complete native lock
change, apart from the root's explicit consumer test dependency. Original Rust
package versions and production consumer manifests/locks are unchanged.

The [published Unix archive](https://crates.io/crates/accesskit_unix/0.24.1)
contains upstream cache-signal correction #819. Source review confirms the tuple
signature change previously carried locally. Registry checksum verification and
runtime private-bus positive/negative controls are required. The old vendor
source, original provenance bytes and import tree remain recoverable from the
frozen baseline above; [the retirement record](accesskit-vendor-retirement.json)
preserves their hashes. Consolidation verification checks those immutable
objects rather than regenerating their baseline hashes. Fyne's separate
shutdown adaptation remains.

[AccessKit 0.25.2](https://crates.io/crates/accesskit/0.25.2)'s production core
source delta documents password-mask values. Its required
[consumer 0.39.2](https://crates.io/crates/accesskit_consumer/0.39.2) changes
password masking/text offsets. Portable tests exercise ordinary Unicode text and
UTF-16 ranges as well as password masking. They do not add password-entry GUI
support or replace native operation evidence. Upstream MIT/Apache texts are
retained with source-revision URLs and hashes; old grants remain intact.

The production binary links the unmodified registry Unix crate. The wire runner
verifies cached registry source checksums, then instruments a temporary copy
with the former regression test. It checks the temporary lock graph against the
production graph, changes only the Unix source origin, and leaves production
sources/lock untouched. The negative control deliberately restores the bad wire
body only in that temporary copy; it must fail the runtime signature assertion,
not compile or skip the test.

The graph still uses zbus 5 and the three unmaintained packages. Their exact
reviewed versions and advisory dispositions remain unchanged. Re-review
**before 2026-11-24** and before release; see [native advisory policy](native-advisories.md).
This narrow replacement does not resolve zbus 6 migration.

## Qualification boundaries

Implementation source snapshot: `eedf8c0d49c8ff86d12e16d8d109887d189a17be`.
Native evidence and exact-head CI results are recorded with this PR. Native
operation, source equivalence and compilation are distinct claims. Earlier
Windows, Linux and Intel Mac records retain their original dependency graphs;
the complete catalog tree is no longer identical to those records.

The original Apple Silicon/Retina handoff is retained unchanged. Returned
feedback can qualify its recorded baseline only; review the new graph delta
before treating it as current. Interactive arm64, actual Retina/display
transitions and broader spoken-reader usability remain pending or unqualified.
No source adaptation or test result closes unavailable hardware gates.

Consumer qualification accepts only a full candidate SHA reachable from reviewed
Scanner main. Final exact-merge consumer checks therefore follow merge; results
are recorded as a PR follow-up. Required real-engine/native tests must execute,
and temporary Cargo patches, tests, locks and embedded binaries must be restored.
Unpublished Organizy changes and private recovery material stay outside public
Scanner.
