# Post-PR #10 qualification — 2026-10-24

Historical PR #11 record. The later [Windows follow-up](windows-shutdown.md)
repairs the toolkit queue and supersedes the Windows timing gate for its tested
revision. Source-equivalence statements below apply to PR #11. Other-host
operation has not been replayed against the later modified toolkit.

[Normalized results and evidence checksums](post-pr10-results.json) distinguish
public Scanner source, private consumer records and remaining platform gates.
Implementation qualification, production adoption, runtime convergence and
release acceptance remain separate.

| Approved follow-up phase | Observed result | Remaining boundary |
| --- | --- | --- |
| 1: consumer refresh | PR #10 merge qualified, then security-main `3540db4` requalified through real source-built compatibility integration, preferred Scanner checks, fixture enrollment/reconciliation, isolated Organizy and unchanged-pin rollback; [private results PR #8](https://github.com/organizyio/organizy-scan/pull/8) | Production pins unchanged; live providers and interactive GUI acceptance are separate |
| 2: Linux hardware | Full native controls, text ranges, live messages, keyboard, lifecycle, restart and quit passed on AMD/Mesa Xwayland with child-only `vblank_mode=0` | Default presentation mode remains unqualified; no driver/compositor settings were changed |
| 3: accessibility | AX/UIA/AT-SPI Unicode caret/range actions passed on available hosts; Linux/Windows native status and affected-folder error events passed | Bounded field scope below; spoken-reader usability and broader selection scenarios remain unqualified |
| 5: maintenance | Upstream fix/release state reviewed; verified Unix backport retained | Removal awaits a published fixed crate and native qualification |

## Exact source scope

[Security PR #12](https://github.com/organizyio/scanner/pull/12) merged patched
ZIP 9.0.5 and the separate, expiring native informational maintenance policy.
[Exact-candidate consumer CI](https://github.com/organizyio/organizy-scan/actions/runs/40092468921)
passed all three jobs at `3540db4466d24fc475b1ebe9f6f46e6664af5cb2`.
The final preferred/compatibility graphs and catalog scanning pipeline match
that qualified security main. All 97 native production files match the separately
exercised `fa7f6889930eadfcf849e5f1fd91d4edda4b6161` GUI source; ZIP metadata
changes do not alter that isolated native graph. Earlier PR #10 consumer results
remain historical evidence for their recorded revision, not evidence for an
unreviewed dependency change.

The [native maintenance dispositions](native-advisories.md) cover exactly three
informational unmaintained notices and require re-review before 2026-11-24.
They do not exempt ZIP vulnerabilities or establish release acceptance.

## Linux rendering scope

The baseline default-mode application stalled in `glfwMakeContextCurrent`.
An owned diagnostic process's native stack placed the wait in Mesa's
`loader_dri3_get_buffers` / `xcb_wait_for_special_event`. A small unmodified Fyne
two-window probe exited successfully, so this evidence does not establish a
universal Fyne or application defect. DRI3-disable and GL-thread diagnostic
attempts did not satisfy the application flow.

The actual accelerated Radeon 8060S/radeonsi renderer, Mesa 25.2.8, passed the
complete replay with Mesa's per-process vsync override disabled. The runner now
makes the mode explicit and requires `glxinfo` to report acceleration:

```sh
python3 scripts/platform-linux-session.py \
  --binary /path/to/scan-gui --fixture /private/new-synthetic-fixture \
  --rendering hardware-no-vsync --orca
```

The equivalent application launch is `vblank_mode=0 /path/to/scan-gui` on the
qualified Mesa host. This disables vsync for that process. It is an opt-in mode;
the application default and host settings stay unchanged. Software rendering
remains a separately identified mode. Native Wayland remains unsupported.

## Native text and feedback

The bridge exposes caret and range state for bounded single-line, non-password
entries and rejects invalid ranges without taking keyboard focus. Native AX
UTF-16 offsets, UIA ranges and AT-SPI Unicode-scalar offsets were exercised with
`aé🙂z`; selecting `é🙂` and collapsing to a caret preserved the field contents.
Tests cover reverse ranges, invalid/cross-node requests, disabled controls and
background modal controls. Range updates do not emit value-edit callbacks.

Navigation, validation and persistence messages now carry polite live semantics.
Actual Linux AT-SPI Announcement and Windows UIA LiveRegionChanged events included
both scan completion and the affected-folder error. Orca ran silently on a private
bus. These are protocol observations, not spoken-reader usability approval.

Three Windows runs retained the live observer through native Quit and exited zero
in 140–175 ms without Invoke errors. An earlier detach-before-quit run exceeded
the eight-second bound; a longer diagnostic run later exited zero. That ordering
case remains an explicit timing/recovery follow-up, rather than being declared
fixed by the successful active-observer runs. Raw failed evidence is retained.

Fyne's public API does not expose a selection anchor. Ambiguous repeated-text
ranges are omitted, not assigned a guessed direction. Multiline/CRLF fields,
password fields and entries over 4096 Unicode scalars do not advertise range
actions. [Dependency maintenance and retirement criteria](native-a11y-maintenance.md)
retain the corresponding follow-ups.

## Continuing gates

- Interactive macOS arm64 and actual Retina 2x hardware remain unavailable and pending. Intel AX evidence and CI cross-builds do not close them.
- Default Linux presentation and Windows observer-detach-before-quit timing remain unqualified.
- Broader selection mapping and spoken-reader usability require their own evidence.
- Private history, recovery archives and raw host details remain outside public Git. The primary unpublished Organizy checkout was not used.
- No preferred/compatibility runtime convergence, production dependency adoption, publication or release acceptance is established.
