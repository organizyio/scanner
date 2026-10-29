# Scanner: Apple Silicon and Retina qualification

Prepared: **2026-10-29**.

Send this guide together with both attachments:

- [Qualification kit ZIP](scanner-macos-arm64-qualification-kit.zip)
- [ZIP checksum](scanner-macos-arm64-qualification-kit.zip.sha256)

The kit builds Scanner locally. It contains no compiled application, private history, or previous test evidence. Its screenshot, pointer-testing, process-exit and packaging mechanics were checked on an Intel/1× Mac. Apple Silicon and Retina qualification remain the tests requested below.

## Rerun after the accessibility event-loop fix

Use a **new workspace**, retaining the previous failed run unchanged. This kit
pins the merged fix and updated AccessKit dependency graph. Do not reuse the
old build or change timeouts. The automated stage first records serial AX
latency for the main and Settings windows (median below 10 ms, snapshots below
5 seconds), then requires the full GUI flow and all 16 shutdown cases.
Latency-only diagnostics do not satisfy screenshot or pointer qualification.

Repeat both main-window and Settings 2× captures and real pointer clicks.
The Settings click must actually occur; geometry alone is insufficient.
Keep **2×↔1× transitions pending** when no 1× display is available. Do not
alter screen/security settings without the operator's consent. No VoiceOver
replay is requested. Keep both failed and successful evidence private.

## Objective and boundaries

Please qualify Scanner on your Apple Silicon Mac, record the actual results, capture screenshots, and return a compressed evidence package. These instructions apply to either a human tester or their Hermes Agent.

Use this reviewed public source:

```text
Repository: https://github.com/organizyio/scanner
Commit: @SCANNER_REVISION@
```

We need two independently recorded capabilities:

- **Apple Silicon:** native arm64 builds and actual application operation.
- **Retina:** operation, accessible geometry and real pointer clicks with the application window on a display reporting an actual **2× backing scale**.

Display transitions between 2× and 1× are an additional test if both displays are available. Missing hardware must be reported as pending.

This work gathers implementation and platform evidence. **It does not approve a release, publish artifacts, upgrade production consumers, or consolidate runtime engines.**

## 1. Prepare the Mac

Use a physical Apple Silicon Mac and an ordinary, signed-in graphical user account.

Before starting:

- Connect power and keep the Mac awake and unlocked during GUI tests.
- Allow space for compiler downloads and build caches—approximately 15 GB free is a useful starting point.
- Keep existing projects and repositories untouched.
- Use a native terminal and Python, without Rosetta.
- Run tests without `sudo`.
- Use only the synthetic scan folders created by the kit.
- Do not enable VoiceOver for this handoff; native accessibility testing is included without spoken-reader qualification.

If Hermes runs only through SSH without access to the signed-in graphical session, perform the GUI stages from the Mac’s local Terminal.

Check Apple’s command-line tools:

```bash
xcode-select -p
```

If they are missing:

```bash
xcode-select --install
```

Finish the Apple installer before continuing. Then verify:

```bash
xcrun --find clang
xcrun --find swiftc
git --version
```

## 2. Select native Python 3.12 or newer

Try the existing Python:

```bash
export SCANNER_PY="$(command -v python3)"

"$SCANNER_PY" -c 'import platform,sys; print(sys.version); print(platform.machine()); assert sys.version_info >= (3,12); assert platform.machine() == "arm64"'
```

The architecture must print:

```text
arm64
```

If Python is missing or too old, an existing native Homebrew installation can provide it:

```bash
brew install python@3.12
export SCANNER_PY="$(brew --prefix python@3.12)/bin/python3.12"
```

This is Homebrew’s documented Python installation command. [Python formula](https://formulae.brew.sh/formula/python@3.12)

If Homebrew is not installed and the Mac owner chooses to install it, use its official installer:

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

Follow its prompts. On Apple Silicon, make it available in the current terminal:

```bash
eval "$(/opt/homebrew/bin/brew shellenv)"

brew install python@3.12
export SCANNER_PY="$(brew --prefix python@3.12)/bin/python3.12"
```

[Official Homebrew installation instructions](https://brew.sh/)

Repeat the Python verification:

```bash
"$SCANNER_PY" -c 'import platform,sys; print(sys.version); print(platform.machine()); assert sys.version_info >= (3,12); assert platform.machine() == "arm64"'
```

If it reports `x86_64`, stop and resolve the Rosetta/native-Python setup. Do not count an Intel process as native arm64 qualification. If the required tooling cannot be installed on this Mac, report the tooling gate as blocked.

## 3. Save and verify the attachments

Save these exact filenames in Downloads:

```text
scanner-macos-arm64-qualification-kit.zip
scanner-macos-arm64-qualification-kit.zip.sha256
```

Verify the download:

```bash
(
  cd "$HOME/Downloads" &&
  shasum -a 256 -c scanner-macos-arm64-qualification-kit.zip.sha256
)
```

Expected output:

```text
scanner-macos-arm64-qualification-kit.zip: OK
```

If verification fails, stop and obtain the attachments again.

## 4. Extract the kit and create a fresh workspace

Run:

```bash
export SCANNER_KIT_CONTAINER="$(mktemp -d "$HOME/scanner-qualification-kit.XXXXXX")"

ditto -x -k \
  "$HOME/Downloads/scanner-macos-arm64-qualification-kit.zip" \
  "$SCANNER_KIT_CONTAINER"

export SCANNER_KIT="$SCANNER_KIT_CONTAINER/scanner-macos-arm64-qualification-kit"

mkdir -p "$HOME/scanner-qualification-runs"

export SCANNER_RUN="$(mktemp -d "$HOME/scanner-qualification-runs/arm64.XXXXXX")"

"$SCANNER_PY" "$SCANNER_KIT/qualify.py" init \
  --workspace "$SCANNER_RUN"
```

This creates a new public Scanner checkout at the pinned commit. It does not reuse or reset an existing checkout.

Save the terminal variables so the work can resume after a terminal restart:

```bash
"$SCANNER_PY" - "$SCANNER_RUN" "$SCANNER_KIT" "$SCANNER_PY" <<'PY'
from pathlib import Path
import shlex
import sys

run, kit, python = sys.argv[1:]
target = Path(run).parent / "latest.env"
values = {
    "SCANNER_RUN": run,
    "SCANNER_KIT": kit,
    "SCANNER_PY": python,
}
target.write_text(
    "".join(f"export {key}={shlex.quote(value)}\n"
            for key, value in values.items())
)
print("Resume file:", target)
print("Workspace:", run)
PY
```

After reopening Terminal, restore them with:

```bash
source "$HOME/scanner-qualification-runs/latest.env"
```

## 5. Install isolated compilers and build Scanner

Run these commands individually:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" setup \
  --workspace "$SCANNER_RUN"
```

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" build \
  --workspace "$SCANNER_RUN"
```

The kit:

- Downloads checksum-verified Go **1.24.13**.
- Installs Rust **1.88** into this run’s own tool directories.
- Keeps compiler caches inside the workspace.
- Builds the native accessibility adapter, Scanner GUI, catalog CLI and inspection helpers.
- Verifies that the application and helpers are arm64 binaries.
- Records compiler versions, binary hashes and source identity.

The first build can take substantial time. Each command prints its log path.

Do not run `go mod tidy`, update dependencies, edit the source, or change the pinned commit.

## 6. Grant the normal macOS permissions

Check permissions:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" permissions \
  --workspace "$SCANNER_RUN"
```

If permissions are missing, request the normal macOS prompts:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" permissions \
  --workspace "$SCANNER_RUN" \
  --request
```

The Mac owner should grant the application that actually launches the helpers—typically Terminal, iTerm or the relevant Hermes host application:

- **System Settings → Privacy & Security → Accessibility**
- **System Settings → Privacy & Security → Screen & System Audio Recording**, or **Screen Recording**, depending on macOS version.

These permissions allow the targeted keyboard/accessibility interactions and application-window screenshots. [Apple accessibility instructions](https://support.apple.com/guide/mac-help/allow-accessibility-apps-to-access-your-mac-mh43185/mac), [Apple recording-permission instructions](https://support.apple.com/guide/mac-help/control-access-to-screen-and-system-audio-recording-mchld6aa7d23/mac)

If macOS requires restarting the terminal application:

1. Restart it.
2. Restore the variables:

   ```bash
   source "$HOME/scanner-qualification-runs/latest.env"
   ```

3. Repeat the permission check.

The file below must report both authorization fields as `true`:

```text
$SCANNER_RUN/evidence/permissions.json
```

Do not reset TCC databases, bypass privacy controls, or grant Full Disk Access for this test. If permission cannot be provided, report the affected tests as blocked/pending.

## 7. Run automated build, filesystem and compatibility checks

Run once in this workspace:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" checks \
  --workspace "$SCANNER_RUN"
```

This includes:

- Preferred Rust formatting, linting, documentation, tests and CLI build.
- Catalog characterization and Go checks.
- Native GUI/accessibility and Fyne queue race tests.
- macOS native lifecycle regression.
- Compatibility engine and Go wrapper tests.
- Actual integration with the locally built embedded engine.
- Public history and source-integrity verification.
- Unicode and symlink behavior.
- Actual read-denied paths.
- Disappearing files and cancellation.
- Checkpoint inventory reconstruction.
- Native CLI interruption.

Review:

```text
$SCANNER_RUN/evidence/checks-summary.json
$SCANNER_RUN/evidence/cli/cli-report.json
$SCANNER_RUN/evidence/filesystem/report.json
$SCANNER_RUN/evidence/signals/report.json
```

`checks-summary.json` should contain:

```json
"passed": true
```

POSIX advisory locking is reported separately; it must not be represented as Windows mandatory-sharing qualification.

If this phase fails, preserve its logs. Do not rerun the entire phase in the same workspace or delete failed fixtures to obtain a green result.

## 8. Run the automated native GUI qualification

Run:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" automated \
  --workspace "$SCANNER_RUN" \
  --label arm64-native
```

**During this command, do not type, click, switch applications, lock the screen, or run another desktop automation.** The test intentionally brings Scanner forward and sends real keyboard events.

It exercises:

- Native controls and readable status.
- Unicode text selection and caret behavior.
- Folder error, recovery and selection.
- Profile changes and restoration.
- Keyboard tree selection, focus and matching details.
- Pause, resume and cancellation.
- Settings/checkpoint preservation across process restart.
- Sixteen shutdown cases involving attached, detached and absent accessibility observers.
- Settings and modal windows.
- Active filesystem work, Quit ordering and checkpoint flush.
- Natural exit zero within the existing shutdown deadlines.

The test saves screenshots automatically.

Expected result files, relative to the workspace:

```text
evidence/automated/arm64-native/flow/report.json
evidence/automated/arm64-native/shutdown/summary.json
evidence/automated/arm64-native/accepted.json
```

The shutdown summary must contain sixteen successful cases.

A forced cleanup, timeout, missing screenshot or unexpected native invocation error is not a pass.

## 9. Start the separate Retina and visual-review session

Run:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" manual-launch \
  --workspace "$SCANNER_RUN" \
  --label retina-a
```

The command prints the owned process identity and synthetic scan root.

Keep this same Scanner process running for the following display tests.

Move the **Scanner window completely onto the Retina display**. Resize it if necessary so the entire window fits. Keep it visible.

A Retina display somewhere on the computer is insufficient—the tested application window must actually occupy that display.

Run:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" capture \
  --workspace "$SCANNER_RUN" \
  --label main-2x \
  --title Scanner \
  --control "Add Folder" \
  --expected-scale 2 \
  --click
```

This checks:

- The owned window’s actual display backing scale.
- That almost the entire window occupies that display.
- Native accessible control bounds.
- Native hit testing at the control’s center.
- A real pointer click at those coordinates.
- That the click opens the expected folder picker.
- Application-window screenshots before and after the click.

It then cancels the synthetic picker.

Inspect this workspace-relative directory:

```text
evidence/manual/retina-a/captures/main-2x/
```

`display.json` should report:

```json
"actual_backing_scale": 2
```

`accepted.json` should also report:

```json
"actual_pointer_click": true
```

If the Mac has no actual 2× display, retain Retina qualification as pending. You may still gather a 1× arm64 sample:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" capture \
  --workspace "$SCANNER_RUN" \
  --label main-1x-only \
  --title Scanner \
  --control "Add Folder" \
  --expected-scale 1 \
  --click
```

Do not use `FYNE_SCALE` or enlarged screenshots to simulate Retina hardware.

If a 2× display is available but a capture fails, inspect its scale and hit-test results. Do not relabel an observed geometry or pointer failure as missing hardware.

## 10. Check keyboard focus and the Settings window

Capture actual keyboard navigation:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" action \
  --workspace "$SCANNER_RUN" \
  --label tree-focus \
  --keys "cmd+l,down,space"
```

Inspect the resulting screenshots:

```text
evidence/manual/retina-a/actions/tree-focus/
```

Confirm:

- Keyboard focus is visibly distinguishable.
- Exactly the intended folder row is selected.
- The details correspond to that selected folder.
- Labels and status are readable.
- Controls are not unexpectedly clipped or overlapping.

Open Settings:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" action \
  --workspace "$SCANNER_RUN" \
  --label open-settings \
  --keys "cmd+comma"
```

Move **Settings itself** completely onto the Retina display. Keep its top controls visible.

Run:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" capture \
  --workspace "$SCANNER_RUN" \
  --label settings-2x \
  --title Settings \
  --control "Browse root 1" \
  --expected-scale 2 \
  --click
```

Inspect:

```text
evidence/manual/retina-a/captures/settings-2x/
```

This independently checks the secondary window’s display scale, accessible bounds and actual pointer response.

If only 1× hardware exists, use a new label and `--expected-scale 1`, and explicitly retain Retina as pending.

Afterward, close **only the Settings window** using its red window-close control. Keep the main Scanner window running.

## 11. Test display transitions when both 1× and 2× displays exist

This must use the **same running Scanner process**, without restarting it.

The initial `main-2x` capture provides the first 2× sample.

Move Scanner fully onto the 1× display, wait briefly, then run:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" capture \
  --workspace "$SCANNER_RUN" \
  --label main-1x \
  --title Scanner \
  --control "Add Folder" \
  --expected-scale 1 \
  --click
```

Move the same window back completely onto the Retina display, wait briefly, then run:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" capture \
  --workspace "$SCANNER_RUN" \
  --label main-back-2x \
  --title Scanner \
  --control "Add Folder" \
  --expected-scale 2 \
  --click
```

Review all three samples. Confirm that:

- Their process IDs match.
- Scale changes from 2 → 1 → 2.
- Native bounds and hit testing remain valid.
- Real pointer clicks continue opening the correct picker.
- The window remains readable and correctly laid out.

If there is no 1× display, record that specific gate as pending:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" record \
  --workspace "$SCANNER_RUN" \
  --case display-transitions \
  --status pending \
  --notes "No physical 1x display was available; same-process 2x-to-1x-to-2x transitions were not tested."
```

## 12. Exit the manual session naturally

With the main Scanner window available:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" manual-quit \
  --workspace "$SCANNER_RUN"
```

Review:

```text
evidence/manual/retina-a/exit.json
evidence/manual/retina-a/accepted-quit.json
```

Expected results include exit code zero and no forced cleanup.

If natural Quit fails and the owned test application is still running, preserve the failure and use:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" cleanup \
  --workspace "$SCANNER_RUN"
```

Cleanup targets only the recorded test process after checking its identity. **A cleanup result is not a successful shutdown qualification.**

## 13. Review screenshots and write the actual findings

Open the evidence directory:

```bash
open "$SCANNER_RUN/evidence"
```

Review at least:

| Area | Evidence |
| --- | --- |
| Initial GUI and readable controls | Automated flow `initial/` |
| Unicode entry and selection | Automated flow `unicode/` |
| Actionable folder error | Automated flow `error/` |
| Folder selection and keyboard focus | Automated flow `tree/` and manual `tree-focus/` |
| Pause, resume and cancellation | Automated flow state folders |
| Restart/checkpoint disclosure | Automated flow `restarted/` |
| Retina main window and actual click | Manual `main-2x/` |
| Retina Settings window and actual click | Manual `settings-2x/` |
| Display transitions, if available | Manual `main-1x/` and `main-back-2x/` |

A black image, thumbnail, obscured window or screenshot without the expected state does not satisfy visual evidence.

Record observations using this template, replacing the status and notes with what was actually observed:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" record \
  --workspace "$SCANNER_RUN" \
  --case keyboard-focus-review \
  --status passed \
  --notes "REPLACE WITH ACTUAL OBSERVATIONS" \
  --evidence "manual/retina-a/actions/tree-focus"
```

Allowed statuses:

```text
passed
failed
pending
not_run
```

Record separate cases for:

- Visual readability and clipping.
- Visible keyboard focus and matching folder details.
- Main-window Retina pointer behavior.
- Settings-window Retina pointer behavior.
- Display transitions, or why they were unavailable.
- Any unexpected behavior or failures.
- Screenshot review and privacy review.

Do not claim a pass merely because a command returned zero. Do not leave placeholder notes in the submitted report.

Case IDs cannot be overwritten. Use a new case ID for a subsequent observation and retain earlier failures.

## 14. Handle failures honestly

If a command fails:

1. Preserve its log and fixture directory.
2. Record what happened, including the failing command and visible state.
3. Capture a window-only screenshot if possible.
4. Check for missing permissions, an interrupted foreground session, missing tools or unavailable hardware.
5. Do not change source, relax deadlines or retry repeatedly until green.
6. If a retry is justified after identifying an environmental interruption, use a fresh label and retain both attempts. The full `checks` phase should use a fresh workspace for a retry.

If automatic screenshots fail but the test window is visible, the human may take a window-only screenshot with macOS’s screenshot controls and place that PNG under this run’s evidence directory. Avoid capturing unrelated desktop content.

Missing permissions and hardware should be reported as blocked/pending, with the reason. An observed application or test failure should remain recorded as a failure.

## 15. Package and return the results

Once all owned manual windows have exited or been explicitly cleaned up:

```bash
"$SCANNER_PY" "$SCANNER_KIT/qualify.py" package \
  --workspace "$SCANNER_RUN"
```

The command creates a compressed ZIP and checksum under:

```text
$SCANNER_RUN/share/
```

Open that directory:

```bash
open "$SCANNER_RUN/share"
```

The package includes:

- Source, host and compiler identity.
- Binary hashes and architecture checks.
- Test commands, logs and result JSON.
- Automated and manual screenshots.
- Display-scale, native-bound and pointer evidence.
- Operator observations.
- A screenshot index.
- Per-file checksums and an archive checksum.

It excludes the source checkout, `.git` history, compilers, build caches and compiled application binaries. Symlinks and special files are not followed into the package.

Review the contents before sending. Logs and application screenshots may contain the macOS username in synthetic file paths; **return the package privately rather than publishing it**.

Send back:

1. The generated **results ZIP**.
2. Its matching **`.zip.sha256` file**.
3. A short message stating:
   - Mac model and macOS version.
   - Whether native arm64 tests passed.
   - Whether actual 2× tests passed.
   - Whether 2× ↔ 1× transitions were tested.
   - Any failed, blocked or pending cases.

Keep the workspace until we have received and reviewed the package.

**Leave release acceptance, production adoption and runtime convergence unapproved. We will review this evidence and update the qualification gates separately.**
