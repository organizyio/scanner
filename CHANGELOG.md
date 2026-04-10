# Changelog

All notable changes to this workspace are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- CI aligned with [Rust Project Primer](https://rustprojectprimer.com/ci/index.html): `rust-toolchain.toml`, `cargo --locked`, `typos`, `taplo`, `cargo-deny`, `cargo doc`, concurrency groups.
- Weekly maintenance workflow (`cargo machete`, `cargo outdated`).
- Release preflight (locked tests, MSRV 1.88, minimal-versions check, optional `cargo-semver-checks` for `scanner`).
- `CHANGELOG.md`, `publish-scanner.yml`, and crates.io-oriented metadata on the `scanner` crate (`publish = false` until workspace deps are publishable).

## [0.2.1] - 2026-04-10

### Changed

- Public `scanner` facade crate and `scanfs` CLI layout; workspace checks and docs.
