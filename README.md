# Scanner workspace

[![crates.io](https://img.shields.io/crates/v/scanner-core.svg)](https://crates.io/crates/scanner-core)
[![docs.rs](https://docs.rs/scanner-core/badge.svg)](https://docs.rs/scanner-core)
[![CI](https://github.com/organizyio/scanner/actions/workflows/ci.yml/badge.svg)](https://github.com/organizyio/scanner/actions/workflows/ci.yml)
[![Release](https://github.com/organizyio/scanner/actions/workflows/release.yml/badge.svg)](https://github.com/organizyio/scanner/actions/workflows/release.yml)

Rust workspace for filesystem scanning: a public **`scanner`** library crate and the **`scanfs`** CLI.

Latest release: <!-- release-version:start -->v0.2.1<!-- release-version:end -->

## Layout

| Path | Role |
|------|------|
| [`scanner/`](scanner/) | **Recommended dependency** for Rust code: `scan`, `scan_with_callbacks`, `ScanOptions`, `FileRecord`, `FilterOptions`, `WalkMode`, etc. |
| [`scanfs/`](scanfs/) | Command-line tool (`scanfs` binary) |
| [`crates/`](crates/) | Internal crates (`engine`, `walker`, `hash`, …) — not the supported public API |

## Requirements

- **Rust** toolchain: pinned in [`rust-toolchain.toml`](rust-toolchain.toml) (`channel`, `rustfmt`, `clippy`). CI uses the same file via `dtolnay/rust-toolchain@stable` in this directory.
- **Lockfile:** CI and `make check` / `make ci-local` use **`cargo … --locked`** for reproducible builds ([Primer — reproducibility](https://rustprojectprimer.com/ci/index.html#reproducibility)).

## Rust Project Primer map

This workspace follows [Checks](https://rustprojectprimer.com/checks/index.html), [CI](https://rustprojectprimer.com/ci/index.html) / [GitHub Actions](https://rustprojectprimer.com/ci/github.html), and [Releasing](https://rustprojectprimer.com/releasing/index.html):

| Area | What we use |
|------|-------------|
| **Fast tier (PR)** | Matrix (Ubuntu / Windows / macOS): `cargo fmt`, `clippy -D warnings`, `cargo test --locked`, `cargo doc --workspace --no-deps --locked`, build `scanfs --locked`; Ubuntu `lint`: `typos`, `taplo fmt --check`, `cargo deny check`; Ubuntu `validate`: `cargo fetch --locked`, MSRV **1.88** (`cargo +1.88 check -p scanner-core --locked`), nightly `-Zminimal-versions` workspace check, `cargo build --workspace --benches --locked` ([`ci.yml`](.github/workflows/ci.yml)) |
| **Concurrency** | `concurrency` + `cancel-in-progress` on PRs (not `main`) to save matrix minutes |
| **Tool installs** | Pinned versions via `taiki-e/install-action` in CI |
| **Thorough / periodic** | Weekly [`scheduled.yml`](.github/workflows/scheduled.yml): `cargo machete`, `cargo outdated` |
| **Binaries & auto release** | [`release.yml`](.github/workflows/release.yml): **(1)** `workflow_dispatch` or **(2)** after successful **CI** on a **push** to `main` — patch-bump from the last `v*` tag, [`CHANGELOG.md`](CHANGELOG.md) from **conventional commits**, bot commit + tag, then **preflight** (locked tests, **MSRV 1.88**, **minimal-versions**, optional `cargo-semver-checks`), **matrix build** `scanfs`, **GitHub Release** — all in **one workflow** (no PAT; same pattern as inlined GoReleaser after prepare). **(3)** Push a **`v*`** tag yourself → only build + release. Skipped when `HEAD` is already tagged on the CI-driven path. |
| **Crates.io** | Automated in [`release.yml`](.github/workflows/release.yml) job `publish-crate` when repository variable **`PUBLISH_TO_CRATES_IO=true`**; gated by environment **`crates-io`** and secret **`CARGO_REGISTRY_TOKEN`** ([security](https://rustprojectprimer.com/ci/index.html#security)). |

The `scanner-core` crate depends on internal `scanner-*` crates from this workspace. The release workflow publishes those internal crates first, then publishes `scanner-core`, all with locked dependency resolution.
| **Versioning** | [CHANGELOG.md](CHANGELOG.md) (Keep a Changelog); pre-1.0 SemVer: minor bumps may be breaking for `0.x` consumers |

**Branch protection and `release` (prepare step):** If `main` requires pull requests or restricts who can push, the **prepare** job must be allowed to push commits and tags (same run then builds and creates the GitHub Release). Typical options: allow **`github-actions[bot]`** to bypass rules for this workflow, or use a branch policy that permits bot pushes. Without that, **Commit, tag, and push** fails after tests.

### Manual release (tag-only path)

To ship without the automated prepare step (no bot commit on `main`):

1. Bump **`version =`** across **`scanner/`**, **`scanfs/`**, and **`crates/*/`** `Cargo.toml` files (e.g. **`cargo set-version`** from [cargo-edit](https://github.com/killercup/cargo-edit)).
2. **`cargo generate-lockfile`**, **`cargo test --workspace --locked`**, and update **[`CHANGELOG.md`](CHANGELOG.md)**.
3. Commit and merge to **`main`**, then **`git tag v0.2.3`** and **`git push origin v0.2.3`**. That push triggers **[`release.yml`](.github/workflows/release.yml)** preflight → artifacts → GitHub Release.

To disable **only** CI-driven auto bumps: remove or narrow the **`workflow_run`** trigger in [`release.yml`](.github/workflows/release.yml); you can still use **Actions → Release → Run workflow** or manual tags.

**Monorepo note:** If this tree lives inside a larger repo, GitHub only runs workflows from the repository root `.github/workflows/`. Copy or call these jobs from the root workflow if you need them on every PR.

## Using the library

In `Cargo.toml`:

```toml
scanner = { package = "scanner-core", path = "scanner/scanner" }  # adjust path from your crate
```

Example:

```rust
use scanner::{scan, ScanOptions};

fn main() -> Result<(), scanner::ScanError> {
    let mut opts = ScanOptions::default();
    opts.roots = vec!["/path/to/root".into()];
    scan(&opts, &mut std::io::stdout())?;
    Ok(())
}
```

## Using the CLI

From this directory:

```bash
cargo run -p scanfs -- --help
cargo run -p scanfs -- --root /data --format json
```

Environment:

- **`SCAN_FS_SOURCE`** — label for the `source` field in JSON output (default: hostname).

## Development

With **GNU Make** (Git Bash, Linux, macOS):

```bash
make help
make ci-local   # same sequence as CI matrix job (fmt, clippy, doc, scanfs, test; all --locked where applicable)
make check      # fmt-check + clippy + doc + test + build-scanfs
make lint-extra # typos + taplo + cargo deny (install tools locally or rely on CI)
```

Or invoke **cargo** directly:

```bash
cargo fmt --all
cargo fmt --all -- --check
cargo clippy --workspace --all-targets --locked -- -D warnings
cargo doc --workspace --no-deps --locked
cargo test --workspace --locked
cargo build --locked --bin scanfs
```

Release binary:

```bash
cargo build --release --locked -p scanfs
```

## Optional distribution (not wired by default)

If you ship **`scanfs`** as a container or system package, see the Primer: [containers](https://rustprojectprimer.com/releasing/containers.html) (e.g. multi-stage build with **cargo-chef** for layer cache) and [packaging](https://rustprojectprimer.com/releasing/packaging.html) (**cargo-deb** / **cargo-generate-rpm**). Add a `Dockerfile` or deb metadata only when that is a product goal.
