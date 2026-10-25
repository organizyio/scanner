# Linux default rendering: live desktop qualification

Reviewed 2026-10-25. The recorded Ubuntu/GNOME Xwayland host passes default
accelerated rendering with the unchanged application. [Exact-source results](linux-default-results.json)
retain the failed diagnostics and separate source, desktop availability and
implementation/release statuses. Other drivers and unavailable Mac hardware are
separate gates.

## Diagnosis

The earlier full-flow stall was reproduced with the desktop locked and its
screens powered down. `loginctl` reported `LockedHint=yes`, GNOME reported its
screen saver active, and Mutter reported display power-save mode 3. A normal
window-manager activation request was refused. The old runner instead used
`XSetInputFocus`, which can assign X11 focus to an obscured Xwayland client even
when the actual desktop cannot present or accept normal interaction.

Retaining the OpenGL context, explicitly requesting synchronized presentation,
and switching to EGL did not resolve the observed locked/off run. Disabling
vertical synchronization did, but that changes presentation semantics and was
not accepted as a default-mode fix. Those diagnostic overlays remain outside
public source; none is shipped.

The owner then unlocked the host. The same application binary immediately
passed the full default-rendering flow, both with the old observation harness
and with normal window-manager activation. The guarded candidate subsequently
passed five complete runs; the final foreground-check revision passed another
five. Every final run used default rendering without `vblank_mode` or a software
override, with unlocked/powered-on samples and real owned window activation.
This resolves the reproduced default-mode qualification gate for this host;
it does not establish that arbitrary driver or lock/power transitions are safe.

Earlier records lacked live-desktop and compositor-activation proof. Preserve
their native protocol observations and failures, but do not treat their
screenshots or no-vsync successes as proof of normal interactive presentation.
The new record is the authority for this host's live default-mode qualification.

## Correction and regression coverage

The change is in the qualification harness, not the application or graphics
stack. The complete catalog/toolkit tree remains identical to reviewed main
`c16955e76e4768c74d77fa3b4b3ac1a80c0af58d` and the earlier application build.
Consumer pins, contracts, settings/checkpoints and release behavior are unchanged.

Before launching any private bus or GUI, the parent requires an inactive GNOME
screen saver and confirmed display power mode 0. Unknown or unsupported state
keeps qualification pending. Availability is sampled during the run and checked
again at completion. A lock/power failure is sticky; a later unlock cannot erase
it. Desktop probes run outside the deadline-supervision thread so a slow D-Bus
response cannot hide an overdue native action. Worker reports remain distinct
from the final qualified report.

The X11 helper requests ordinary application activation through
`_NET_ACTIVE_WINDOW`. It requires both the window manager's active window and
X11 focus to belong to the selected synthetic window before input and around
capture. It no longer injects focus through `XSetInputFocus`. Refused activation
stops the run; no lock screen, desktop policy or native Wayland restriction is
bypassed. See the [EWMH activation specification](https://specifications.freedesktop.org/wm/1.5/ar01s03.html).

Regression tests cover locked/off/unknown/unsupported states, stale monitoring,
slow probes, sticky failure, false X11-only focus and normal activation messages.
A locked-preflight regression proves no GUI/private-session process is spawned
and no successful report is written. The live flow also opens Settings, captures
its controls/keyboard focus, sends its normal `WM_DELETE_WINDOW` request and
verifies main-window survival and unchanged saved settings.

## Recorded native scope

Five final default-mode runs cover two attached-observer processes, two
observer-detached-before-Quit processes and one silent Orca process. Each run
also performs an actual process restart. Native Unicode caret/ranges,
status/affected-folder Announcement events, picker error/recovery, profiles,
keyboard focus/selection/details, Settings close, pause/resume/cancel and
settings/checkpoint restart passed. Quit required natural exit zero under the
eight-second total deadline; forced cleanup never counted as acceptance.

Screenshots and native cache wire signatures are retained privately. Silent
Orca observed application/error/tree events without cache-signature or parent
lookup warnings. Spoken-reader usability remains a separate qualification.

## Reproduce

Use an exact reviewed application and record its source/binary hashes. On the
existing authorized Xwayland display, leave GNOME unlocked and its display on:

```sh
python3 scripts/platform-linux-session.py --binary /private/evidence/scan-gui \
  --fixture /private/evidence/new-default-attached --rendering default
python3 scripts/platform-linux-session.py --binary /private/evidence/scan-gui \
  --fixture /private/evidence/new-default-detached --rendering default \
  --observer-mode detach-before-quit
python3 scripts/platform-linux-session.py --binary /private/evidence/scan-gui \
  --fixture /private/evidence/new-default-orca --rendering default --orca
```

Use a new fixture for every attempt. Inspect the parent's final
`session-report.json`, `desktop-state.json`, host accessibility before/after
state and per-capture `foreground.json`/`foreground-after.json`. A
`worker-report.json` alone cannot qualify a run. The existing eight-second
native-stage bounds remain enforced independently of desktop sampling.

The availability observer currently supports GNOME. Another desktop requires
an appropriate readiness observer before this harness can qualify it. Native
Wayland remains unsupported. The runner never unlocks the host, inhibits
screen locking or power management, or changes global rendering settings.

Interactive macOS arm64, actual Retina/display transitions and broader reader
gates remain separate. The Mac handoff pinned to `c16955e` remains valid: its
application/toolkit source and Mac harness are unchanged. No production adoption,
runtime convergence, release publication or product release acceptance is implied.
