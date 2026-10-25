# Native platform qualification

Implementation qualification and release acceptance are separate. A successful
cross-build, widget test, or headless run does not qualify interactive Fyne use.
The [qualification register](platform-results.json) records exact tested source
revisions and distinguishes CLI, compilation, interaction and accessibility.

## Reproduce safely

Use an exact reviewed Scanner commit (a PR candidate before merge, or merged
main) and build `tools/project-catalog` commands
`./cmd/scan-cli` and `./cmd/scan-gui` on the target host. Build the isolated native
accessibility library first with `python3 scripts/build-native-a11y.py --check`
from the Scanner root; it uses pinned Rust 1.88 and never downloads a binary. Record the complete source
SHA and binary SHA-256. Do not change production dependency pins.

Use a new private evidence directory outside the checkout for every run:

```sh
python3 scripts/platform-qualification.py prepare --fixture /tmp/scanner-native-run
python3 scripts/platform-qualification.py cli \
  --fixture /tmp/scanner-native-run --binary /path/to/scan-cli
python3 scripts/platform-qualification.py launch \
  --fixture /tmp/scanner-native-run --binary /path/to/scan-gui
```

The synthetic fixture contains Unicode paths, a symlink where supported, a Go
marker and an isolated settings/profile/checkpoint configuration. The runner
sets child-process HOME, USERPROFILE, APPDATA, LOCALAPPDATA and XDG directories;
it never uses or modifies the operator's ScannerApp configuration. Baseline
permission tests verify that access actually fails and restore fixture-only
permissions even on exceptions. Windows sharing tests hold an exclusive handle;
POSIX advisory locking is explicitly not equivalent and is not reported as a
successful locked-file qualification.

On macOS, `swift scripts/platform-macos.swift PID OUTPUT_DIRECTORY tab,space`
records the native accessibility tree, sends keyboard events to the synthetic
process and captures application windows. It requires existing accessibility
and capture permissions; unavailable permissions keep qualification pending.
Use actual visible control interactions and inspect focus/screenshots rather
than inferring behavior from event delivery.

On Linux, `platform-x11.py --pid PID --output OUTPUT_DIRECTORY --keys Tab,space`
targets only the synthetic process's X11/Xwayland windows using the existing
authorized display session. It records app-only screenshots and native AT-SPI
content. It does not grant access, bypass Wayland policies, or qualify native
Wayland windows. Empty AT-SPI content is an accessibility gap, not a pass.

On Windows, run `platform-windows.ps1 -Binary PATH -Fixture PATH` after preparing
the fixture. From SSH it creates a uniquely named temporary task using the
currently signed-in user's interactive token, waits for evidence and removes
that exact task. No passwords, elevated token, host policy changes or persistent
environment changes are required. If the signed-in console is unavailable or
cannot focus the synthetic window, qualification stays pending. The runner
records native UIAutomation, actual keyboard delivery and an app-bounded image.
It stops only the Fyne process it launched.

## Interactive checklist

- Enter/change a synthetic scan root in Settings and validate the profile.
- Start a sufficiently long synthetic workload; observe progress and visible
  keyboard focus. Invoke Pause, Resume and Cancel through actual window controls.
- Restart the process with the same isolated settings, then use Start (Resume).
  Verify settings/checkpoint compatibility and explain residual checkpoint limits.
- Check folder selection, Unicode labels, tree navigation and actionable errors.
- Inspect native accessible names/roles for Start, Pause, Resume, Cancel, Settings,
  roots and status. Visual labels alone do not prove screen-reader accessibility.
- Exercise actual Windows denied ACLs and sharing violations, plus symlinks,
  disappearing files and interruption. Preserve observable partial-result policy.

Keep raw host logs, screenshots and access details in private evidence. Public
results contain synthetic summaries, exact public source revisions and hashes;
never usernames, host aliases, private repository history or recovery material.
Record unsupported scenarios and failed checks honestly. Runtime convergence,
GUI support announcements and release publication remain excluded.

## Native semantics and remaining host gates

The GUI owns a semantic tree through an isolated AccessKit adapter for macOS AX,
Windows UIA and Linux AT-SPI. Actions are delivered through the existing widget
callbacks on the Fyne event loop. Folder and profile selection use app-owned,
named controls. Tree keyboard navigation keeps actual selection, details and
semantic focus aligned. Disabled, removed and background modal actions are
rejected against the current tree. The native adapter must close before its
window is destroyed; concurrent queries and shutdown are separate lifecycle
regressions, not evidence of screen-reader usability.

Linux qualification covers X11/Xwayland. Native Wayland attachment is explicitly
unsupported. An isolated session bus with an in-memory settings backend may be
used to activate AT-SPI without changing the operator's desktop configuration;
such evidence must identify that scope.

No interactive macOS arm64 host is available; the owner explicitly retained that
gate as pending. The available Intel Mac has a 1× display. Automated 1×/2× bounds
transform checks do not qualify actual Retina hit testing or display transitions.
These gates cannot be satisfied by CI compilation or extrapolated from another
host. The qualification register must retain them independently of merged fixes.

Checkpoint continuation returns an inclusive cursor suffix, not a complete
replacement inventory. Compare the deduplicated union of the saved prefix and
resumed suffix with an uninterrupted inventory. Active filesystem calls are not
preempted: cancellation stops subsequent work after the call returns.

## Explicit Linux modes and extended text evidence

The [post-PR #10 record](post-pr10-results.md) qualifies accelerated Mesa/Xwayland
with `--rendering hardware-no-vsync`. The runner verifies acceleration before
launch and records `vblank_mode=0` in the child environment. `--rendering software`
and the unchanged `default` mode are separate claims; host settings are untouched.

The extended Linux runner checks actual Unicode caret/range APIs and scan/error
Announcement events. Windows `-Actions announcements,textselection,keyboard,quit`
checks live-region events, native ranges and quit with the observer attached.
These checks do not establish spoken-reader usability or unavailable macOS
arm64/Retina operation. Raw evidence remains private.

## Windows shutdown repair, 2026-10-25

The [Windows follow-up](windows-shutdown.md) closes the reproduced observer-detach
shutdown gate on its exact candidate after a Fyne queue repair and 26 passing
native runs. Earlier failure evidence remains retained. Additional native Unicode,
keyboard and settings-window Quit checks passed. The shared toolkit adaptation is
compiled and regression-tested across CI hosts; earlier macOS/Linux operational
evidence remains tied to its prior revisions. Native replay on those hosts,
unavailable arm64/Retina hardware and release acceptance remain separate gates.

## Linux and available Mac replay, 2026-10-25

[The follow-up](linux-macos-replay.md) replays the merged Fyne adaptation on
Linux and macOS x86_64, including native lifecycle, keyboard, text and filesystem
checks. Its normalized results distinguish observer-attached and detached
shutdown, explicit accelerated/software modes and the unresolved Linux default
presentation failure. Unavailable macOS hardware and release acceptance remain
separate.
