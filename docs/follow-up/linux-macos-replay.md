# Linux and available macOS native replay

Reviewed 2026-10-25. This replays the Fyne adaptation merged in
[PR #13](https://github.com/organizyio/scanner/pull/13) on the available Intel Mac
and Linux desktop. [Exact revisions, artifact digests and scoped results](linux-macos-results.json)
are separate from release acceptance. Earlier evidence and failed runs remain retained.

## Recorded scope

The application, native adapter and complete catalog/toolkit tree are unchanged
from `f1c09580c3e66ceb3b83601cc5869465f75a6608`. This follow-up changes qualification
harnesses, CI helper compilation and documentation; it does not change scanning,
schemas, profiles, checkpoints, dependency pins or the platform graphics stack.

On macOS x86_64, macOS 15.8.1 with an actual 1x display, the native AX flow covers
folder selection, affected-folder error/recovery, Unicode text selection/caret,
profile switching, visible keyboard focus and matching tree/details, pause,
resume, cancellation, natural Quit, settings preservation and actual process
restart with inclusive partial-checkpoint disclosure. Separate shutdown runs
exercise real AX observers attached through Quit, detached before Quit, and
absent, including Settings and a modal. Active extraction remains non-preemptible;
the active-I/O case must release its owned read and flush the checkpoint before exit.
The accepted active case is a stricter supplementary run: a write-only FIFO
handle proves Scanner's reader is present, the native Quit invocation must return
before read release, and the checkpoint must name that completed FIFO. It replaces
the earlier matrix's time-based active case; the other fifteen cases are unchanged.

Linux x86_64 uses the existing Ubuntu 24.04 Xwayland desktop with a private
memory-only accessibility bus. Accelerated Mesa with explicit child-only
`hardware-no-vsync` and software rendering are distinct passing modes. Native
AT-SPI controls, Unicode ranges, status/error Announcement events, profiles,
keyboard focus/selection/details, pause/resume/cancel, process restart and natural
Quit are asserted. Both observer-attached and detach-before-Quit runs are recorded.
Silent Orca observes application/error/tree events without cache-signature or
parent-lookup warnings; this is not spoken-reader usability evidence.

Both hosts also repeat actual read denial, Unicode/symlink records, disappearing
files, cancellation, saved-prefix plus inclusive-suffix inventory reconstruction
and actual CLI SIGINT/SIGTERM. POSIX advisory locks are explicitly inapplicable
to the separate Windows mandatory-sharing claim.

## Qualification corrections

The Linux parent now supervises monotonic, nonce-bound native stages rather than
starting an exit timeout only after a potentially blocking native action returns.
Observer detach and total Quit are each bounded by eight seconds; late completion
retains timestamps and cannot erase a missed deadline between polls. Regression
tests include a genuinely blocked child and late completion.

The Mac matrix found a helper timeout with Settings open. A timing trace shows
over seven seconds spent reading all attributes of both windows, followed by
control lookup; no Quit invocation had occurred before the deadline. Pure Quit
requests now omit that unrelated full-tree snapshot. Earlier control screenshots
remain required, and lookup, invocation and natural exit still share the same
eight-second supervisor budget. The trace records invocation and return explicitly.
The failed run and diagnostic stacks are retained; forced cleanup never qualifies.

## Reproduce

Build the source-pinned native adapter and GUI as described in
[platform qualification](platform-qualification.md). Compile the public helpers:

```sh
swiftc scripts/platform-macos.swift -o /private/evidence/ax-helper
swiftc scripts/platform-macos-observer.swift -o /private/evidence/ax-observer
python3 scripts/platform-macos-session.py \
  --fixture /private/evidence/new-flow --binary /private/evidence/scan-gui \
  --helper /private/evidence/ax-helper --revision FULL_REVIEWED_SOURCE_SHA
python3 scripts/qualify-macos-shutdown.py \
  --binary /private/evidence/scan-gui --helper /private/evidence/ax-helper \
  --observer /private/evidence/ax-observer --evidence /private/evidence/new-matrix \
  --revision FULL_REVIEWED_SOURCE_SHA --binary-sha256 EXPECTED_BINARY_SHA256
```

The Mac matrix stops on its first failure and requires a new evidence directory
for a full replay. It does not enable VoiceOver or request permissions. Existing
accessibility/capture authorization and foreground ownership remain required.

On the authorized Linux display, use fresh synthetic fixture paths:

```sh
python3 scripts/platform-linux-session.py --binary /private/evidence/scan-gui \
  --fixture /private/evidence/new-attached --rendering hardware-no-vsync
python3 scripts/platform-linux-session.py --binary /private/evidence/scan-gui \
  --fixture /private/evidence/new-detached --rendering hardware-no-vsync \
  --observer-mode detach-before-quit
```

`--orca` adds the existing silent reader to the private session. The parent checks
that the host accessibility state is unchanged. Neither this runner nor the Mac
runner modifies user ScannerApp settings. Raw logs, screenshots, personal paths
and access details remain outside public Git.

<a id="remaining-gates"></a>

## Historical remaining gates at PR #14

Default Linux presentation remains **failed/unqualified**, not unavailable hardware.
The full flow again stalls with the application in GLFW `MakeContextCurrent`;
the same binary passes with explicit no-vsync or software rendering. A small
Fyne window-lifecycle diagnostic exits normally even in default mode, so it does
not establish a complete root cause or justify a general graphics workaround.
Default-mode failures and the unsuccessful restricted debugger attachment remain
private evidence. No host driver, compositor, security policy or global rendering
setting was changed, and no fallback silently changes default behavior.

Interactive macOS arm64 and actual Retina 2x/display transitions remain pending
because suitable hardware is unavailable. Native Wayland remains unsupported.
Broader spoken-reader use, ambiguous repeated-text anchors, multiline/CRLF,
password and oversized range semantics remain outside this qualification.

Phase 4 remains incomplete. Windows retains its separately recorded PR #13
scope. Production consumer adoption, runtime convergence, release publication
and product release acceptance remain separate decisions.

## Subsequent Linux investigation

This page retains PR #14's historical observations. [The live desktop
follow-up](linux-default-rendering.md) records the later diagnosis and five
passing guarded default-mode runs. Earlier Linux runs lacked proof that the
actual desktop was unlocked/powered on and that the window manager had activated
the app. Their native protocol observations remain evidence; their screenshots
and no-vsync passes do not qualify normal interactive/default presentation.
Unavailable macOS gates and release acceptance remain separate.
