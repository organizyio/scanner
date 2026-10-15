# Executable and data contracts

| Component | Executable/interface | Output and important differences |
| --- | --- | --- |
| Preferred Scanner | `scanner-core` (library `scanner`), `scanfs` | Nested versioned records; existing schema identifiers retained |
| Compatibility engine | `scan` | Flat NDJSON; boolean `--format` means MIME sniffing |
| Compatibility Go wrapper | `scan-fs` | Flat normalized NDJSON or plain/table; `--sniff-format` forwards engine `--format` |
| Project catalog | `scan-cli catalog`, `scan-git` | Git/project inventory JSON file; progress on stdout |
| Project deep scan | `scan-cli fs`, catalog `scan-fs` | Files/folder aggregates; independent schema from either Rust engine |
| Fyne | `scan-gui` | Existing Go pipeline/controller and `ScannerApp/settings.yaml` |

Legacy launcher mappings: `scan-projects` → `scan-git`, `scan-export` → catalog `scan-fs`, `scanner` → `scan-gui`. Separate launcher archives preserve these documented mappings without automatically modifying PATH.

The flat and nested schemas are not interchangeable. The facade exports, original schema URLs, package identities, profiles, flags, defaults, error channels and exit behavior remain unchanged. SHA-256, full/partial xxHash64, MIME, EXIF extras and integer widths retain component-specific behavior. Existing null/omitted semantics remain component-specific.

`SCAN_ENGINE_PATH` overrides the wrapper engine. `SCAN_FS_SOURCE` is preserved: the Go wrapper uses whitespace literally; Rust treats blank source as absent. Both normalized CLIs use the literal first root for `device`. Go filepath extension handling of dotfiles differs from Rust. The wrapper table buffers until completion. The Go flag parser, engine clap parser and Rust scanfs parser retain their individual help/error semantics.

## Migration baseline and subsequent discovery corrections

The consolidation preserved the original Go catalog behavior at its immutable
migration baseline. The separately reviewed [phase 2 implementation](../follow-up/README.md)
corrects discovery without rewriting import evidence or changing JSON shapes,
settings locations, profiles or checkpoints:

- Valid `.git` files for linked worktrees/submodules are recognized through read-only Git inspection.
- Failed status inspection leaves `dirty` null; nil and typed-nil optional loggers are safe.
- Git commands disable optional locks and filesystem-monitor hooks, honor cancellation and have a ten-second timeout.
- Exported remotes remove HTTP credentials and URL query/fragment values, redact malformed credential-bearing forms, and remove SSH passwords. Ordinary SSH, SCP and local paths are retained.
- Catalog and deep scan share the union of markers, including Go/Rust/Java. Existing catalog labels win conflicts: Gradle remains Kotlin. Alphabetical primary labels do not represent language dominance.
- Unavailable creation times are empty strings rather than modification-time substitutes.
- Skipped/inaccessible paths, hashing failures and incomplete inspection produce stderr or GUI diagnostics. Successful completion can still contain partial results. Catalog SIGINT writes partial JSON and returns 130; SIGTERM retains its OS default. Deep-scan exit conventions remain unchanged.
- GUI refreshes use the UI dispatcher with coalescing and lifecycle cancellation; hiding to the tray preserves the running application. Compilation and automated lifecycle tests do not qualify interactive GUI operation.

## Remaining limitations

- Preferred Scanner now uses a bounded 256-record queue with caller-thread callbacks and cooperative cancellation. All workers join before return. Active extraction, OS calls and callbacks are not preemptible. The earlier compatibility engine still enqueues without a bound during synchronous traversal; its execution behavior is unchanged. See the [execution specification and measurements](../follow-up/README.md).
- Symlink, exclusion and partial-result behavior is not replaced with managed Organizy traversal.
- Resume is lexical and inclusive of the checkpoint path. It is not a filesystem snapshot or transactional restart. Deep scan retains collected results in memory.
- Native GUI operation, keyboard/accessibility support and real Windows ACL denial require separate evidence. Compilation alone does not qualify those claims.

Runtime convergence, consumer dependency adoption and release acceptance remain
separate decisions. Existing released component artifacts retain their original
behavior; these source corrections do not replace or republish those artifacts.

The wrapper currently can exit 1 when interrupted while its child is blocked, despite a separate code path for 130. Original and relocated binaries both returned 1 in the characterized blocked-child case. No stronger interruption guarantee is claimed.

## Independently qualified security adaptation

Before distribution, root and compatibility builds use lopdf 0.45.0 (with Rust 1.88-compatible AES 0.9.2) and crossbeam-epoch 0.9.20 to satisfy existing security checks ([prerequisite PR](https://github.com/organizyio/scanner/pull/2)). Original-versus-relocated behavior was characterized before this adaptation and is rechecked afterwards. A valid one-page PDF retains its count; deeply nested hostile PDFs are now rejected without aborting the process. This intentional correction addresses [RUSTSEC-2026-0187](https://rustsec.org/advisories/RUSTSEC-2026-0187) and [RUSTSEC-2026-0204](https://rustsec.org/advisories/RUSTSEC-2026-0204). It is not runtime convergence, an Organizy dependency upgrade, or release acceptance.

## Read-only discovery enforcement

A pre-fix regression reproduced `git status` refreshing a tracked repository's index cache during catalog discovery. Discovery now sets `GIT_OPTIONAL_LOCKS=0`, which [Git documents](https://git-scm.com/docs/git#Documentation/git.txt-codeGITOPTIONALLOCKScode) as preventing that side effect. The tracked-index, configuration and worktree characterization tests verify this behavior. This was the explicit read-only migration constraint. The subsequent phase 2 corrections described above add gitdir-file recognition, truthful error reporting and shared marker rules without rewriting that historical evidence. Unix inaccessible-directory behavior is characterized with actual denied access; Windows ACL qualification remains separate from the CLI build/integration matrix.
