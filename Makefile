# Scanner Rust workspace — library (`scanner`), CLI (`scanfs`), internal crates.
#
# Run from this directory (`scanner/`). Toolchain: see rust-toolchain.toml (also used by CI).

SHELL := /bin/bash

.PHONY: help fmt fmt-check clippy doc test build-scanfs ci-local check lint-extra deny minimal-versions clean

help:
	@echo "Scanner workspace targets:"
	@echo "  make fmt           — cargo fmt --all"
	@echo "  make fmt-check     — fmt check (CI-style)"
	@echo "  make clippy        — clippy with -D warnings (--locked)"
	@echo "  make doc           — cargo doc --workspace --no-deps (--locked)"
	@echo "  make test          — cargo test --workspace (--locked)"
	@echo "  make build-scanfs  — cargo build --bin scanfs (--locked)"
	@echo "  make ci-local      — same sequence as .github/workflows/ci.yml matrix job"
	@echo "  make lint-extra    — typos + taplo + cargo deny (tools must be on PATH)"
	@echo "  make deny          — cargo deny check"
	@echo "  make minimal-versions — nightly -Zminimal-versions check (matches release preflight)"
	@echo "  make check         — fmt-check + clippy + doc + test + build-scanfs (all --locked)"
	@echo "  make clean         — cargo clean"

fmt:
	cargo fmt --all

fmt-check:
	cargo fmt --all -- --check

clippy:
	cargo clippy --workspace --all-targets --locked -- -D warnings

doc:
	cargo doc --workspace --no-deps --locked

test:
	cargo test --workspace --locked

build-scanfs:
	cargo build --locked --bin scanfs

# Mirrors CI job `test` (matrix) — see Rust Project Primer CI / reproducibility (--locked).
ci-local: fmt-check clippy doc build-scanfs test

check: fmt-check clippy doc test build-scanfs

lint-extra: typos taplo-check deny

typos:
	typos

taplo-check:
	taplo fmt --check

deny:
	cargo deny check

minimal-versions:
	rustup toolchain install nightly --profile minimal --no-self-update
	cargo +nightly -Zminimal-versions check --workspace --all-targets

clean:
	cargo clean
