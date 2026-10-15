# scan-projects

Monorepo with three ways to inspect disks:

| Component | Command | Role |
| --------- | ------- | ---- |
| **Unified CLI** | `scan-cli` | Subcommands: `catalog` (project inventory) and `fs` (deep walk) → JSON |
| **Catalog (shim)** | `scan-git` | Same as `scan-cli catalog` (legacy name `scan-projects`) |
| **Deep scan (shim)** | `scan-fs` | Same as `scan-cli fs` (legacy name `scan-export`) |
| **Deep scan (GUI)** | `scan-gui` | Fyne desktop app; `internal/controller` drives the same deep pipeline as `scan-cli fs` |

**Legacy binary names:** `scan-projects` → `scan-git`, `scan-export` → `scan-fs`, `scanner` → `scan-gui`.

**Layout:** `internal/pipeline` orchestrates deep scan (`scan.Scan` + `analysis.PostProcessFolders`) and catalog inventory. `internal/cli` holds flag parsing for `scan-cli` and the shim binaries. Under that: `internal/catalog` + `internal/gitmeta` + `internal/markers` for project discovery; `internal/scan` (with `walk`, `filter`, `hash`, `checkpoint`) for the deep walk; `internal/analysis` for project-root interpretation; `internal/storage` for YAML profiles. `scan.StartScanStream` remains available for streaming file events.

---

## Setup

```bash
go mod tidy
```

---

## Build

### Unified CLI (recommended)

```bash
go build -o scan-cli ./cmd/scan-cli
```

Cross-platform release builds emit **only** `scan-cli` (see `build.sh` / `make build-all`). For the current OS, `make build-local` also places **`scan-git`** and **`scan-fs`** shims in `dist/` with the same flags as before.

### Legacy shim binaries (optional)

```bash
go build -o scan-git ./cmd/scan-git
go build -o scan-fs ./cmd/scan-fs
```

### GUI (native toolchain required)

```bash
go build -o scan-gui ./cmd/scan-gui
```

The Fyne app needs a working OpenGL/driver stack on your OS. Cross-compiling the GUI is not covered here; build on the target platform or follow [Fyne](https://developer.fyne.io/) cross-compile docs.

**Linux / WSL (Debian, Ubuntu, etc.)** — install headers and pkg-config so CGO can compile `go-gl/gl` and `go-gl/glfw` (fixes `Package gl was not found` and `X11/Xlib.h: No such file or directory`):

```bash
sudo apt update
sudo apt install -y build-essential pkg-config \
  libgl1-mesa-dev libegl1-mesa-dev \
  libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev xorg-dev
```

Then ensure CGO is on (`export CGO_ENABLED=1`) and rebuild. To **run** the GUI on WSL2 you need a display (e.g. [WSLg](https://github.com/microsoft/wslg) on recent Windows 11, or an X server).

---

## Build for many platforms (scan-cli only)

```bash
./build.sh
```

Binaries land in `dist/` as `scan-cli-*`. The GUI is **not** included in that script. Use `make build-local` on your machine if you also want `scan-git` / `scan-fs` shims in `dist/`.

---

## Usage: catalog (`scan-cli catalog` or `scan-git`)

```bash
./scan-cli catalog \
  --root ~/dev \
  --root /Volumes/ext-ssd/dev-archive \
  --output machines/laptop.json
```

Example JSON entry:

```json
{
  "name": "carrio-api",
  "path": "/Users/arman/dev/github/org-carrio/carrio-api",
  "stacks": ["dotnet", "python"],
  "primary_stack": "dotnet",
  "is_git": true,
  "has_origin": true,
  "remotes": [
    { "name": "origin", "url": "git@github.com:carrio-org/carrio-api.git" }
  ],
  "host": "github.com",
  "owner": "carrio-org",
  "repo_slug": "carrio-api",
  "current_branch": "main",
  "local_branches": ["main", "develop"],
  "tracking_branch": "origin/main",
  "dirty": false
}
```

---

## Usage: deep scan CLI (`scan-cli fs` or `scan-fs`)

```bash
go build -o scan-cli ./cmd/scan-cli
./scan-cli fs -root D:\dev -o deep.json -no-hash \
  -exclude .git -exclude node_modules
```

Flags: repeatable `-root`, repeatable `-exclude` (directory **names** only), `-output` / `-o`, `-no-hash`, `-max-file-mb`, `-no-projects`, `-no-default-excludes` (skip built-in excludes such as `node_modules`; by default those are merged with your `-exclude` list).

Run `scan-cli -h` for top-level help and `scan-cli fs -h` / `scan-cli catalog -h` for command flags.

---

## Usage: GUI (`scan-gui`)

```bash
go build -o scan-gui ./cmd/scan-gui
./scan-gui
```

Profiles (YAML) define scan roots, excludes, hashing, checkpoints — see `internal/storage/profiles.go` for fields.

---

## Multi-machine workflow (catalog)

On your dev machine:

```bash
cd scanner/tools/project-catalog
go mod tidy
./build.sh
```

Copy the **`scan-cli`** binary for the target OS/arch from `dist/` (e.g. `scan-cli-windows-amd64.exe`, `scan-cli-macos-arm64`, `scan-cli-linux-amd64`). Optional: use `make build-local` and copy the shim binaries from `dist/` if you rely on the `scan-git` / `scan-fs` names.

Run on any machine, for example:

```bash
./scan-cli-macos-arm64 catalog \
  --root ~/dev \
  --root /Volumes/SSD1/dev-archive \
  --output machines/macos.json
```

For a deep scan on that host:

```bash
./scan-cli-linux-amd64 fs -root ~/dev -o deep.json -no-hash
```

## Discovery correctness and partial results

Catalog and deep scans share filename/extension markers, including Go, Rust,
Java and the existing Kotlin label for Gradle. Detected stacks are sorted;
the primary stack/project kind is an alphabetical display label, not a measure
of language dominance. Linked worktree and submodule `.git` files are validated
with read-only Git inspection. Git subprocesses disable optional locks and
filesystem monitor hooks, honor cancellation and have a ten-second timeout.
Failed status inspection leaves `dirty` null rather than claiming a clean tree.
Exported HTTP URLs omit user information, query strings and fragments; SSH
passwords are removed while ordinary SSH usernames and addressing are retained.
Malformed credential-bearing URL forms are redacted.

An unavailable creation time is the empty `created_at` string; modification time
is never substituted as birth time. Skipped/inaccessible paths and hashing
failures produce stderr warnings and partial-result status in the GUI. Existing
JSON envelopes and exit conventions are retained: successful completion can
contain partial results. Check stderr/status before treating an inventory as
complete. Settings, profiles and checkpoint formats and locations are unchanged.
These corrections do not qualify GUI operation or authorize a release.
