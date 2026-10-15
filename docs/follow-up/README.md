# Scanner correctness, execution and qualification

Owner-approved implementation specification, 2026-10-15. Sole owner: @ArmanAvanesyan.

## Acceptance matrix

| Phase | Required evidence | Implementation | Platform/consumer qualification | Release |
| --- | --- | --- | --- | --- |
| 1: contracts and licensing | Immutable baseline checks, golden fixtures, measured synthetic baseline, owner licensing grant | Merged ([PR #6](https://github.com/organizyio/scanner/pull/6)) | Baseline only | Not accepted |
| 2: Go discovery | Worktrees/submodules; timeout/cancel; null unknown dirty; safe logger; sanitized remotes; shared markers; honest timestamps; visible partial results | Implemented; in review | Pending | Not accepted |
| 3: preferred execution | Queue bound 256; caller-thread callbacks; 100ms cancellation polling; stage checks; joined workers and failure cleanup | Pending | Pending | Not accepted |
| 4: platforms and Fyne | CLI matrix plus actual ACL denial; interactive GUI/keyboard/accessibility on macOS, Linux and Windows | Pending | Pending; compile success is insufficient | Not accepted |
| 5: consumers | Exact-main candidate; real engine; fixture enrollment/inventory; isolated published Organizy Cargo patches; rollback; Archivist boundary | Pending | Pending; production pins unchanged | Not accepted |

Phase 1 merged before phase 2 behavioral changes. Merge focused phase PRs only after exact-head checks pass. Missing native evidence keeps qualification pending rather than converting compilation into acceptance. Runtime convergence and publishing/releases are excluded.

## Contracts and intentional corrections

Keep catalog JSON shapes, Rust facade signatures, schemas, module identities, settings locations, profiles and checkpoints. Preserve old release assets and immutable migration hashes. [Change dispositions](changes.json) explicitly explain post-consolidation source changes; they do not rewrite migration baselines.

Phase 2 preserves nullable `dirty` but returns null for failed Git inspection. It recognizes valid gitdir files, caps each read-only Git command at ten seconds, and propagates cancellation. Exported HTTP remotes drop credentials/query/fragment; diagnostic output cannot reveal credentials. Catalog and deep scan share the union of marker rules; existing catalog labels win conflicts (Gradle remains Kotlin). Sorted primary stack is a deterministic display label, not measured dominance. Unknown birth time is an empty string. Warnings reach stderr and existing GUI status; partial-result exit conventions remain unchanged. Catalog SIGINT writes partial JSON and returns 130; SIGTERM retains the OS default. GUI refreshes are coalesced on the Fyne UI dispatcher and stop on actual window/app shutdown; hiding to the tray does not create or stop pollers. Automated lifecycle tests and native compilation do not establish interactive qualification.

Phase 3 uses a traversal producer with a synchronous 256-record queue and caller-thread callback consumer. Callbacks need no Send bound. Receiver disconnection unblocks producers on error, cancellation or unwind; every worker is joined. Cancellation is checked before traversal, each extraction stage and delivery, with 100ms receive polling. Active extraction, blocking OS calls and blocked user callbacks are not preemptible. Old compatibility engine/wrapper behavior is unchanged.

## Validation and evidence

Run `make check`, pinned isolated engine checks, real-engine Go integration, headless Go catalog tests, component archive tests and `python3 scripts/verify-consolidation.py`. Native matrices and interactive evidence are recorded separately. Evidence uses synthetic paths; private recovery and host access details never enter this public repository.

`capture-contracts.py` captures normalized golden output, command/exit behavior and synthetic baseline measurements. Normalize paths, timestamps, machine identifiers and ordering only; keep payload values and JSON types. No performance improvement is inferred from a single local baseline. Existing migration tests also characterize profiles and checkpoint recovery.

Consumer qualification uses exact candidate SHA ancestry from reviewed Scanner main and records resolved revisions. Organizy tests run on published main with temporary Cargo patches for every Scanner package, never in the unpublished primary checkout. Merge harnesses/results only; keep production pins and locks unchanged. Archivist's independent engine is reviewed, not replaced.
