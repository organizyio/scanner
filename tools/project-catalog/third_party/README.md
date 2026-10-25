# Reviewed Fyne shutdown repair

`fyne/` is the complete public Go-module snapshot of Fyne v2.7.3, authenticated
by the module checksum retained in `../go.sum`. `fyne-provenance.json` records
the original archive, commit and every original/current file hash. There is no
Git history or private source in this copy.

The local replacement retains package identity, API and dependency versions.
Only the GLFW shutdown loop is adapted: run stopped hooks on the app goroutine,
seal queue submissions atomically, and acknowledge all accepted waiters through
end-of-stream, including input not yet published to the output channel.
The added internal queue has deterministic and race regression coverage.

Original Fyne code and authorship remain BSD-3-Clause (`fyne/LICENSE`, `AUTHORS`).
Scanner additions are MIT (`fyne/LICENSE-SCANNER`). Both notices must accompany
GUI distributions. Consolidation verification rejects unrecorded snapshot drift.

Related upstream evidence: [shutdown report](https://github.com/fyne-io/fyne/issues/6021)
and [stopped-hook ordering proposal](https://github.com/fyne-io/fyne/pull/6442).
That proposal is closed without merge; this is a reviewed local adaptation,
not an upstream release fix. Remove it only after verifying and qualifying a
published replacement containing both the queue handoff and drain corrections.

Run `go test -mod=readonly -race ./internal/async` inside `fyne/` for regressions.
Scanner's GUI CI compiles the replacement on macOS, Windows and Linux. Native
Windows replay establishes only its recorded host scope. Other platform gates,
production dependency adoption, runtime convergence and release acceptance remain
separate.
