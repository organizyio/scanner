# Scanner correctness, execution and qualification

Owner-approved implementation specification, 2026-10-15. Sole owner: @ArmanAvanesyan.

## Acceptance matrix

| Phase | Required evidence | Implementation | Platform/consumer qualification | Release |
| --- | --- | --- | --- | --- |
| 1: contracts and licensing | Immutable baseline checks, golden fixtures, measured synthetic baseline, owner licensing grant | Merged ([PR #6](https://github.com/organizyio/scanner/pull/6)) | Baseline only | Not accepted |
| 2: Go discovery | Worktrees/submodules; timeout/cancel; null unknown dirty; safe logger; sanitized remotes; shared markers; honest timestamps; visible partial results | Merged ([PR #8](https://github.com/organizyio/scanner/pull/8)) | Regression and native CLI checks passed; GUI gates remain separate | Not accepted |
| 3: preferred execution | Queue bound 256; caller-thread callbacks; 100ms cancellation polling; stage checks; joined workers and failure cleanup | Merged ([PR #7](https://github.com/organizyio/scanner/pull/7)) | Deterministic execution and documented consumer flows qualified; broader native gates remain | Not accepted |
| 4: platforms and Fyne | CLI matrix plus actual ACL denial; interactive GUI/keyboard/accessibility on macOS, Linux and Windows | Native bridge, lifecycle and keyboard corrections merged (PRs #10, #13–15); host gates remain | [Exact host results and unavailable-host gates](platform-qualification.md) | Not accepted |
| 5: consumers | Exact-main candidate; real engine; fixture enrollment/inventory; isolated published Organizy Cargo patches; rollback; Archivist boundary | Qualification harness merged | Exact reviewed candidate qualified for documented flows; private results retained; production pins unchanged | Not accepted |

Phase 1 merged before phase 2 behavioral changes. Merge focused phase PRs only after exact-head checks pass. Missing native evidence keeps qualification pending rather than converting compilation into acceptance. Runtime convergence and publishing/releases are excluded.

## Current qualification — 2026-10-29

[The current register](current-qualification.json) identifies superseding evidence
without rewriting historical runs or their hashes. Scanner main at review is
`fcb049522e3043ad71c7f8e5d0a3669beadbaa39` (PR #22). Phases 1–3 are
implemented; phase 5 has scoped candidate evidence and an unchanged production
pin. Phase 4 remains incomplete.

| Host or gate | Current qualification | Remaining boundary |
| --- | --- | --- |
| Linux x86_64, GNOME/Xwayland | [Five guarded default accelerated runs](linux-default-rendering.md), including Settings keyboard/native close, lifecycle/restart, attached/detached observers and silent Orca | Only this live desktop/renderer; other desktops need readiness observers; native Wayland unsupported |
| Windows x86_64 | [26 native shutdown runs](windows-shutdown.md), UIA controls/text, keyboard, Settings, restart and observer ordering | Recorded host scope; broader spoken-reader usability unqualified |
| Intel macOS, actual 1x | [Native AX flow, shutdown matrix and actual active-I/O checkpoint shutdown](linux-macos-replay.md) | Does not qualify arm64 or Retina |
| macOS arm64 | [Returned baseline partially passes; event-loop fix and rerun kit](macos-ax-latency.md) | Updated graph and complete shutdown/Settings pointer replay pending |
| Actual Retina 2x and display transitions | Baseline main-window real 2x click passed; Settings click requires rerun | 2x/1x transitions pending without 1x display |
| Broader spoken-reader usability | Native protocol evidence only | Unqualified; additional spoken testing deferred |

The older Linux GPU/default-presentation failure and Windows observer-detach
failure are retained below as historical observations. PR #13 repaired Windows
shutdown, PR #14 replayed Linux/Intel Mac, and PR #15 established live Linux
desktop/foreground proof. They are not current unresolved defects on those
qualified hosts. The Linux application and rendering defaults were unchanged.

[Latest dependency review](maintenance-png-review-2026-10-26.md) records published sources at its baseline. The
[AccessKit replacement qualification](accesskit-metadata-qualification.md)
records the later graph delta and vendor retirement; Fyne adaptation and the
November 24 review cutoff remain. Consumer refresh results are tracked separately from GUI qualification.
[The latest consumer completion](https://github.com/organizyio/scanner/issues/18#issuecomment-6407572436)
records Scanner `82986118edf3c24fab0fa2deb1ef07b2957aa2f2`: three passing exact-candidate
CI jobs, required embedded-engine integration and two reconciled prototype files
with zero pending batches. Isolated published Organizy resolved all eight Scanner
packages to that candidate; worker tests/build, PNG facade checks in the real
worker graph, all three native managed tests and unchanged-pin rollback passed.
The [previous refresh](https://github.com/organizyio/organizy-scan/pull/9) and its
original evidence remain historical. Exact private refs and raw evidence stay
private. The unrelated interactive vault test remains an explicit skip; live
providers and other Organizy consumer platforms are outside that report.
[PNG decoding limits](png-decoding-limits.md) is a separate follow-up with its own
validation and reviewed-main consumer result. macOS-arm64 CI CLI execution cannot
qualify the pending interactive hardware gate.

Production adoption, runtime convergence, publication and release acceptance
remain excluded.

## Contracts and intentional corrections

Keep catalog JSON shapes, Rust facade signatures, schemas, module identities, settings locations, profiles and checkpoints. Preserve old release assets and immutable migration hashes. [Change dispositions](changes.json) explicitly explain post-consolidation source changes; they do not rewrite migration baselines.

Phase 2 preserves nullable `dirty` but returns null for failed Git inspection. It recognizes valid gitdir files, caps each read-only Git command at ten seconds, and propagates cancellation. Exported HTTP remotes drop credentials/query/fragment; diagnostic output cannot reveal credentials. Catalog and deep scan share the union of marker rules; existing catalog labels win conflicts (Gradle remains Kotlin). Sorted primary stack is a deterministic display label, not measured dominance. Unknown birth time is an empty string. Warnings reach stderr and existing GUI status; partial-result exit conventions remain unchanged. Catalog SIGINT writes partial JSON and returns 130; SIGTERM retains the OS default. GUI refreshes are coalesced on the Fyne UI dispatcher and stop on actual window/app shutdown; hiding to the tray does not create or stop pollers. Automated lifecycle tests and native compilation do not establish interactive qualification.

Phase 3 uses a traversal producer with a synchronous 256-record queue and caller-thread callback consumer. Callbacks need no Send bound. Receiver disconnection unblocks producers on error, cancellation or unwind; every worker is joined. Cancellation is checked before traversal, each extraction stage and delivery, with 100ms receive polling. Active extraction, blocking OS calls and blocked user callbacks are not preemptible. Old compatibility engine/wrapper behavior is unchanged.

Phase 4 preserves the Go pipeline and existing settings/checkpoint identities.
The GUI now exposes app-owned controls through AX/UIA/AT-SPI and aligns folder
tree keyboard selection with visible details. Cancel remains `canceling` until
workers finish; blocked I/O is not preempted. Checkpoint continuation is labeled
as an inclusive partial inventory. Hashing changes apply to subsequent files,
including zero as an unlimited size; live project detection is applied when
folder classification finishes, including canceled partial scans. Folder/profile
selection uses named ordinary controls, reports persistence/navigation failures
and rejects stale asynchronous results and stale native actions. These changes
require actual native qualification; implementation merge does not close absent
macOS arm64 or actual Retina display gates.

## Validation and evidence

Run `make check`, pinned isolated engine checks, real-engine Go integration, headless Go catalog tests, component archive tests and `python3 scripts/verify-consolidation.py`. Native matrices and interactive evidence are recorded separately. Evidence uses synthetic paths; private recovery and host access details never enter this public repository.

`capture-contracts.py` captures normalized golden output, command/exit behavior and synthetic baseline measurements. Normalize paths, timestamps, machine identifiers and ordering only; keep payload values and JSON types. No performance improvement is inferred from a single local baseline. Existing migration tests also characterize profiles and checkpoint recovery.

Consumer qualification uses exact candidate SHA ancestry from reviewed Scanner main and records resolved revisions. Organizy tests run on published main with temporary Cargo patches for every Scanner package, never in the unpublished primary checkout. Merge harnesses/results only; keep production pins and locks unchanged. Archivist's independent engine is reviewed, not replaced.

## Preferred execution measurements

[Phase 3 measurements](phase3-measurements.json) use the same 4096-file synthetic fixture as the [baseline](baseline-measurements.json), with normalized CLI contracts exactly equal. On one local debug run, baseline versus candidate elapsed time was 0.214s / 0.190s, first record 0.041s / 0.017s and peak RSS 10,792,960 / 9,162,752 bytes. Interruption after the first record returned 130 in both runs (0.0019s / 0.0172s). These single-run observations establish neither a performance improvement nor a worst-case cancellation bound. Deterministic tests establish early delivery, the 256-record queue bound and worker cleanup independently of timing. Active extraction, OS calls and callbacks remain non-preemptible.

<a id="phase-4-completion-qualification"></a>

## Historical phase 4 completion candidate (before PRs #11–15)

[The platform register](platform-results.json) retains the earlier baseline and adds development evidence separately. Native filesystem probes now cover disappearing files, symlink hashing, cancellation and reconstruction from a saved prefix plus an inclusive resumed suffix. Native `scan-cli fs` interruption retains its existing exit-zero partial-output convention; this does not change the catalog command's separate signal contract.

| Interactive host | Recorded qualification | Remaining gate |
| --- | --- | --- |
| macOS x86_64, 1x display | Final native AX picker/value/focus/error and real permission warning; keyboard tree/details and distinct focus; retained earlier lifecycle/restart evidence with unchanged controller/scan code | Native Quit/Cmd+Q and active-extraction checkpoint shutdown passed on17ce; actual 2x display unavailable; additional reader behavior remains separately scoped |
| Linux x86_64, Xwayland | Final native AT-SPI text/actions, picker/error, profiles, keyboard tree/details, pause/resume/cancel and actual process restart; silent Orca app/error/tree navigation; correct native cache wire signatures | At this historical candidate: software rendering passed; accelerated/default gate lacked later live-desktop proof. Superseded by the current register |
| Windows x86_64 | Final UIA picker/value/focus/error, Unicode roots, profiles, keyboard tree/details, distinct focus and process restart/settings/partial resume; earlier lifecycle evidence with unchanged controller/scan code | Spoken reader/caret behavior is outside the recorded scope |
| macOS arm64 | No interactive evidence from the available Intel host | Pending interactive arm64 host |

The private Linux bus preserves the existing desktop's accessibility settings. It exercises the real native protocol and actual graphical window, with its session scope recorded in evidence. The final native text/status and Linux cache corrections were verified; the register retains their earlier failures and exact-source evidence separately. GUI compilation, platform qualification, runtime convergence and release acceptance remain separate.

The final register identifies CLI qualification at `d572dec`, shutdown/refresh
qualification at `17ce6cf`, and their scoped source equivalence to candidate
`79c62ae`. Native Quit passed on all available hosts; macOS and Windows shortcuts
also exited successfully. macOS active-I/O Quit waited for read release and
checkpoint flush. No Fyne wrong-thread diagnostics occurred in those final
probes. The earlier shutdown failure remains historical evidence. Interactive
macOS arm64 remains explicitly pending; release acceptance remains separate.

<a id="post-pr-10-follow-up"></a>

## Historical post-PR #10 follow-up

[2026-10-24 qualification](post-pr10-results.md) records the consumer refresh,
explicit accelerated Linux mode, bounded native text ranges and feedback events,
and upstream maintenance review. At that revision, default Linux presentation
and Windows observer-detach timing were pending. Later records supersede those
two gates for their tested scope; unavailable macOS and broader reader gates
remain open. The original evidence remains intact.

[Windows shutdown repair and exact-source results](windows-shutdown.md) record
the subsequent Fyne queue correction and 26 passing native runs. At that point,
Linux default presentation and other-host replay were still pending; PRs #14–15
subsequently supplied the recorded host evidence. Unavailable macOS gates remain.

<a id="linux-and-available-mac-replay-after-pr-13"></a>

## Historical Linux and available Mac replay after PR #13

[2026-10-25 native replay](linux-macos-replay.md) records the unchanged merged
application with its Fyne shutdown adaptation on Linux and the available Intel
Mac. The runners distinguish observer ordering, actual native operation and
filesystem checks from compilation. That record predates the live-desktop
investigation; its Linux default failure is superseded within PR #15 scope.
Unavailable macOS arm64/Retina and broader reader gates remain pending. Production adoption, runtime convergence and release acceptance
are unchanged.

## Linux default-mode gate after live desktop verification

[The live desktop follow-up](linux-default-rendering.md) qualifies default
accelerated rendering on the recorded Ubuntu/GNOME Xwayland host with unchanged
application source. The reproduced stall ran with a locked, powered-off desktop.
The runner now requires live desktop state and real window-manager activation;
five final default-mode runs passed. Pending macOS/Retina evidence and broader reader
gates remain separate; phase 4 and release acceptance are not complete.

## Returned evidence and scale preparation

[The offline returned-evidence verifier](returned-evidence.md) checks the existing
Apple Silicon/Retina handoff format without extracting or executing its content.
It reports integrity and coverage separately from owner review; hardware gates
remain pending until actual returned evidence is reviewed.

[The preferred Scanner scale harness](scale-testing.md) source-builds an isolated
probe for repeated multiroot, slow-consumer, cancellation and failure-recovery
runs. Measurements are synthetic observations, not performance improvement,
GUI qualification, consumer adoption, runtime convergence or release acceptance.

## Mixed workload follow-up

[Mixed workload qualification](mixed-workloads.md) adds large-file hashing,
DOCX inflation/XML and PDF parsing to the synthetic scale corpus. The original
unsupported-PNG evidence is retained. [PNG qualification](png-perceptual-hashing.md)
selects the PNG decoder and requires exact perceptual values in new runs.
Consumer adoption and hardware/release acceptance remain separate.

## PNG resource hardening

[PNG decoding limits](png-decoding-limits.md) bounds encoded input, dimensions
and decoded output while preserving hash goldens, error variants and consumer
pins. The [fresh dependency review](maintenance-png-review-2026-10-26.md) records
published AccessKit candidates separately from the unchanged qualified native
graph and review deadline.

## AccessKit replacement and metadata resource hardening

[The published AccessKit follow-up](accesskit-metadata-qualification.md) records
registry provenance, upstream cache-wire correction, ordinary/password consumer
regressions and native replay. The runtime vendor source is replaced by its
published fix; immutable historical provenance and regression evidence remain.
The native zbus maintenance deadline is unchanged. Old host records retain their
original dependency graph; unavailable Apple Silicon/Retina gates stay pending.

[Preferred metadata budgets](metadata-decoding-limits.md) bound encoded input,
DOCX expansion/XML, PDF page traversal and EXIF export without changing public
signatures or schemas. Parser limits are not aggregate RSS/CPU guarantees.
Compatibility implementations and production consumer pins remain unchanged.
