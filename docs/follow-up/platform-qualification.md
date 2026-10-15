# Native platform qualification

Implementation qualification and release acceptance are separate. A successful
cross-build, widget test, or headless run does not qualify interactive Fyne use.
The [qualification register](platform-results.json) records exact tested source
revisions and distinguishes CLI, compilation, interaction and accessibility.

## Reproduce safely

Use source from reviewed Scanner main and build `tools/project-catalog` commands
`./cmd/scan-cli` and `./cmd/scan-gui` on the target host. Record the complete source
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
