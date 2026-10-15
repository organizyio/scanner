# Scanner native accessibility bridge

This isolated, non-publishing Rust static library exposes the catalog GUI's
app-owned semantic tree through AccessKit's native AX, UIA and AT-SPI adapters.
It does not alter Scanner traversal, records, Go profiles or checkpoints.

Build before compiling a CGO catalog binary, from the Scanner repository root:

```sh
python3 scripts/build-native-a11y.py --check
```

On Windows use `python` if that is the installed command. The helper builds the
x86_64 GNU target for MinGW and copies its library to the stable cgo link path;
compiling with the default MSVC target is not sufficient for a MinGW Go build.

The Go package `internal/nativea11y` links this target-matched static library by
default. A CGO-disabled build provides an explicit unavailable adapter, never a
successful qualification. This library is outside the root Cargo workspace and
is not a crates.io publication target. Do not track `target/` binaries. Go does not fingerprint external static archives
in its build cache: use `go build -a` / `go test -a` for manual native GUI checks.
Component packaging includes the exact library hash in its link flags to force
a fresh link when this source-built library changes.

## Semantic and threading contract

Each window has its own adapter and tree. Node 1 is the window root; other IDs
are nonzero, unique and stable within the window. Every node must reach root 1.
Snapshots include the full tree, logical content-relative bounds, focus and
current values/states. Native queries read owned immutable snapshot values;
no query calls into the Go UI or waits for its event loop. The Go adapter applies
canvas scale, tracks native window focus, and dispatches queued native actions
asynchronously through `fyne.Do`. Closing stops polling and invalidates queued
Go callbacks before releasing native resources. Close the adapter before releasing
the corresponding native window.

Supported roles: window, group, button, text, entry, checkbox, combobox, option,
tree, treeitem and status. Actions are click, focus, set_value, select, expand and
collapse. Unsupported, disabled and unknown-node actions are ignored. The action
queue is bounded to 256 requests. Status nodes can request polite announcements.

macOS calls attach/update/close on AppKit's main thread, patches the GLFW window
focus forwarder and attaches to its content view. Windows uses the manual adapter
and forwards WM_GETOBJECT through a WndProc hook, preserving GLFW's prior procedure;
it avoids AccessKit's pre-show-only convenience adapter. Linux uses AT-SPI and
X11 geometry/focus. Native Wayland attachment fails explicitly; a Wayland surface
is never interpreted as an X11 handle.

Native accessibility is application behavior and must be qualified with native
inspectors, actions and assistive technologies. Successful compilation and model
tests do not constitute accessibility or release acceptance. The app must expose
all relevant controls and keep semantic focus, values and widget actions aligned.

AccessKit dependencies retain their MIT OR Apache-2.0 licenses and applicable
upstream notices. This bridge's owner-authored code uses Scanner's MIT grant.

Windows owner-thread lifecycle regression (native MinGW console):

```sh
gcc -std=c11 -O2 native-a11y/tests/windows_lifecycle.c -luser32 -o windows-lifecycle.exe
./windows-lifecycle.exe
```

This regression uses a real Win32 window/message loop and an adapter test double
for deterministic reentrant-query cleanup. It does not replace real UIA semantic
inspection and actions. Owner-thread dispatch uses queued GetMessage tasks;
timed-out tasks are refcounted and never leave dangling posted pointers. If the
window owner has terminated or cannot pump, shutdown retains native resources to
process exit instead of destroying them from another thread.

macOS native owner-close regression (run in an actual graphical session):

```sh
clang -std=c11 -fblocks native-a11y/tests/macos_lifecycle.m \
  -framework AppKit -framework Foundation -o /tmp/scanner-macos-lifecycle
/tmp/scanner-macos-lifecycle
```

Linux checked X11 stale-window regression (actual X11 or an isolated Xvfb):

```sh
gcc -std=c11 -O2 native-a11y/tests/linux_lifecycle.c -lxcb -lpthread \
  -o /tmp/scanner-linux-lifecycle
/tmp/scanner-linux-lifecycle
```

These native lifecycle tests use explicit Rust adapter test doubles to make
reentrancy deterministic. Native AX/UIA/AT-SPI content and actions require the
separate real-library qualification runs. macOS geometry multiplies the current
native backing factor and Fyne scale before AccessKit converts pixels to AX
points; changing displays refreshes that factor. Empty geometry remains omitted.
