# Compatibility scan engine

Reviewed owner-authored source snapshot, licensed under MIT; private Git history remains private. This isolated Cargo workspace preserves the earlier `scan` CLI and flat NDJSON records. It is excluded from Scanner's default workspace and crate publication.

Build from this directory: `cargo +1.88 build --locked --release -p scan`.
The boolean `--format` sniffs MIME; it does not select an output format.
No streaming/cancellation improvement is implied. See [compatibility](../../docs/consolidation/compatibility.md).
