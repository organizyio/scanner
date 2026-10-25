# Windows shutdown qualification follow-up

Implementation candidate, 2026-10-25. Native acceptance remains pending until the
full exact-source matrix passes. Release acceptance remains separate.

## Demonstrated cause and correction

Repeated runs reproduced an exit failure after observer removal and successful
UIA Quit invocation. The stalled Go stack has the app goroutine waiting for Fyne
lifecycle events while its event goroutine awaits an acknowledgement from the
already stopped graphics function queue. A diagnostic-only binary captured this
stack; it is excluded from qualification.

The [reviewed Fyne v2.7.3 adaptation](../../tools/project-catalog/third_party/README.md)
seals submission atomically with shutdown and drains all accepted input through
end-of-stream. Stopped hooks execute synchronously on the app goroutine before
sealing, preserving cleanup and preferences ordering. The deterministic negative
control restores the old length-based drain and strands an accepted callback;
the corrected race tests release every waiter. Public Scanner interfaces and
schemas, settings/checkpoints and production consumer pins remain unchanged.

## Runner contract

`platform-windows.ps1` keeps observer-attached behavior as its default. Explicit
`-ObserverMode detach-before-quit` and `none` select the other cases;
`-ObserverCycles 10` verifies native error delivery after every reattachment.
The outer same-user interactive-task supervisor observes monotonic stage records
without scheduler RPC during the deadline. Both observer removal and total Quit
(including lookup/invocation, exit and code verification) are bounded by eight
seconds. Blocked invocation cannot hide before an exit wait. Invocation errors,
forced cleanup or late natural exit cannot count as acceptance.

After a recorded failure only, an optional `-DebuggerPath` selects GNU GDB for
bounded stack capture. Diagnostic time does not extend acceptance. PID/start-time
and run-ID checks restrict actions to owned processes/tasks; no host settings are
changed. Use fresh synthetic fixture directories and retain failed reports.

`qualify-windows-shutdown.ps1` requires a new evidence directory, the exact binary
SHA-256 and source commit. It runs 10 detached, 5 attached, 5 detached Ctrl+Q,
5 no-observer processes and one 10-cycle reattachment run. Every run must pass.
The helper retains evidence and stops on the first failure; restart the complete
matrix after a relevant correction.

Raw stacks, personal paths, screenshots and recovery history stay outside public
Git. Public results will retain only sanitized observations and evidence hashes.
macOS arm64/Retina, Linux default rendering and broader accessibility gates remain
pending. No release is published or accepted through this work.
