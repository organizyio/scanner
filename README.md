# Scanner workspace

[![crates.io](https://img.shields.io/crates/v/scanner-core.svg)](https://crates.io/crates/scanner-core)
[![docs.rs](https://docs.rs/scanner-core/badge.svg)](https://docs.rs/scanner-core)
[![CI](https://github.com/organizyio/scanner/actions/workflows/ci.yml/badge.svg)](https://github.com/organizyio/scanner/actions/workflows/ci.yml)
[![Release](https://github.com/organizyio/scanner/actions/workflows/release.yml/badge.svg)](https://github.com/organizyio/scanner/actions/workflows/release.yml)

Rust workspace for filesystem scanning: a public **`scanner`** library crate and the **`scanfs`** CLI.

Latest release: <!-- release-version:start -->v0.2.3<!-- release-version:end -->

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
| **Binaries & explicit release** | [`release.yml`](.github/workflows/release.yml) runs only on an explicit `workflow_dispatch` or a new `v*` tag. Successful main-branch CI does not release. Preflight, binary builds and publication require a separate release decision. |
| **Crates.io** | Publication requires an explicit dispatch with `publish_crates=true`, repository variable `PUBLISH_TO_CRATES_IO=true`, and the `crates-io` environment/secret gates in [`release.yml`](.github/workflows/release.yml). A documentation or qualification merge does not authorize publication. |

The `scanner-core` crate depends on internal `scanner-*` crates from this workspace. The release workflow publishes those internal crates first, then publishes `scanner-core`, all with locked dependency resolution.
| **Versioning** | [CHANGELOG.md](CHANGELOG.md) (Keep a Changelog); pre-1.0 SemVer: minor bumps may be breaking for `0.x` consumers |

**Branch protection and release preparation:** Preserve existing protections. A release needs its own reviewed version/change-log preparation and the applicable workflow permissions; qualification work does not authorize policy bypasses.

### Manual release (tag-only path)

To ship without the automated prepare step (no bot commit on `main`):

1. Bump **`version =`** across **`scanner/`**, **`scanfs/`**, and **`crates/*/`** `Cargo.toml` files (e.g. **`cargo set-version`** from [cargo-edit](https://github.com/killercup/cargo-edit)).
2. **`cargo generate-lockfile`**, **`cargo test --workspace --locked`**, and update **[`CHANGELOG.md`](CHANGELOG.md)**.
3. After separate release authorization, merge the version preparation and create a new, previously unused `v*` version tag. That push triggers [`release.yml`](.github/workflows/release.yml). Never replace an existing tag or released artifact.

CI-driven automatic bumps are disabled. Do not restore a `workflow_run` release trigger as part of qualification or repository maintenance.

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

## Consolidated scanning components

See the [repository consolidation record](docs/consolidation/README.md) for the retained Go catalog/Fyne application, isolated compatibility engine/wrapper, history/privacy boundary and cutover gates. Preferred interfaces remain `scanner-core` and `scanfs`.

See [ZIP dependency security maintenance](docs/follow-up/zip-security.md) for the
reviewed metadata upgrade and its qualification boundaries.
