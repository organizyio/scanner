# macOS accessibility event-loop correction

The returned Apple Silicon baseline reported serial native AX reads around
33 ms, approximately 7–14 second tree snapshots, and timeouts opening/capturing
Settings before the intended Quit or pointer click. Those failures do not
establish a shutdown deadlock. Raw returned evidence remains private.

The Fyne main thread previously slept in a Go select between 60 Hz event
polls. That left Cocoa's synchronous accessibility IPC waiting for a frame.
On macOS, the idle wait now uses GLFW's finite native event wait, which keeps
Cocoa servicing requests. Frame ticks still control drawing; queued Go work
and shutdown are checked between native waits bounded at 1/60 second. Other
platforms retain blocking Go selection. Existing shutdown sealing is preserved.
This does not preempt active filesystem extraction or change runtime engines.

A controlled Intel macOS comparison with the same serial attribute reader
measured baseline medians of 16.6 ms and main snapshots of 3.6 seconds. The
candidate measured medians of 0.04–0.06 ms and main snapshots of 17–32 ms;
main plus Settings took 38 ms and native Quit 0.75 seconds. The initial
screenshot runs failed window capture; these timings are partial diagnostics,
not successful screenshot qualification. Measurements vary by host and do
not establish Apple Silicon performance or a particular 30 Hz mechanism.

`qualify-macos-ax-latency.py` requires main and Settings coverage, serial-read
median below 10 ms, snapshots below 5 seconds, native exit and no forced
cleanup. It explicitly produces AX-only diagnostics, not visual acceptance.
The full helper retains the existing action budgets and captures. Scan state
assertions observe completion inside the same 20-second action budget, without
repeating input: the old slow snapshots had masked asynchronous scan completion. Returned
verification rejects AX-only substitutions for visual evidence. Ready queue,
frame and shutdown selection has separate regression coverage.

Generate a fresh Apple Silicon kit from the merged revision using
`prepare-macos-rerun-kit.py`. It builds the updated SDK graph and reruns latency,
full flow, all 16 shutdown cases, and actual main/Settings 2× pointer clicks.
Retain the earlier failed run. No retry or longer timeout turns it into a pass.
Display transitions stay pending without 1× hardware. Runtime convergence,
production pin adoption and release acceptance remain separate decisions.

## Local replay — 2026-10-29

On macOS 15.8.1 (24H32), Intel x86_64 with an actual 1× display, the full
screenshot-backed Unicode/folder/profile/tree/pause/resume/cancel/restart flow
passed after the asynchronous-state observation correction. All 16 shutdown
cases passed with natural zero exits and no forced cleanup, including attached
Settings, modal, detached/no-observer and active-read checkpoint cases. Quit
took 0.739–1.023 seconds across that matrix. Representative main, Settings,
Unicode and keyboard-focus captures were visually inspected. Earlier failed
capture/timing attempts remain private evidence; they were not rewritten.

The source-pinned rerun kit and verifier profile are generated only after merge.
The full action budget remains 20 seconds and Quit remains 8 seconds. Arm64
performance and current-graph Retina pointer acceptance need the external rerun.

## Reviewed M1 rerun — 2026-10-30

[The returned M1/Retina evidence](macos-arm64-retina.md) directly tests merged
`92100f46ce0bd1c93a0145df080874e62866b634`. Native arm64 build and all 20 checks,
serial AX latency, full GUI flow and all 16 shutdown cases passed. Both Main
and Settings actual 2× pointer clicks opened their pickers and returned after
Cancel. Median AX reads were 0.017–0.021 ms and gate snapshots 15–31 ms.

This supersedes the earlier pending arm64/static-2× gates only for that recorded
host and revision. The initial failed evidence remains unchanged. Same-process
2×↔1× transitions are still pending without a physical 1× display; broader
spoken-reader usability is unqualified/deferred. Runtime convergence, production
adoption and release acceptance remain separate.
