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

## Preserved limitations

- Preferred Scanner now uses a bounded 256-record queue with caller-thread callbacks and cooperative cancellation. All workers join before return. Active extraction, OS calls and callbacks are not preemptible. The earlier compatibility engine still enqueues without a bound during synchronous traversal; its execution behavior is unchanged. See the [execution specification and measurements](../follow-up/README.md).
- Catalog Git discovery recognizes `.git` directories, not `.git` files used by worktrees/submodules.
- Failed Git status commands can report clean (`dirty=false`); missing Git does not establish a clean repository. Failed lookups can panic if callers supply a typed nil catalog logger; existing CLI supplies a logger.
- Catalog and deep scan have inconsistent stack markers (notably Go/Rust/Java). Primary stack is alphabetic, not measured dominance.
- Some platform creation times in the Go catalog fall back to modification times. They are not reliable birth-time evidence.
- Legacy traversal skips inaccessible entries and can return success/partial results. Symlink, exclusion and error semantics are not replaced with managed Organizy traversal.
- Git subprocesses are read-only but lack timeouts; remote URL fields retain their original raw values. Treat exported local inventory as potentially sensitive when remotes contain credentials.
- Resume is lexical and inclusive of the checkpoint path. It is not a filesystem snapshot or transactional restart. Deep scan retains collected results in memory.

Corrections and convergence need independent specs, tests and consumer qualification.

The wrapper currently can exit 1 when interrupted while its child is blocked, despite a separate code path for 130. Original and relocated binaries both returned 1 in the characterized blocked-child case. No stronger interruption guarantee is claimed.

## Independently qualified security adaptation

Before distribution, root and compatibility builds use lopdf 0.45.0 (with Rust 1.88-compatible AES 0.9.2) and crossbeam-epoch 0.9.20 to satisfy existing security checks ([prerequisite PR](https://github.com/organizyio/scanner/pull/2)). Original-versus-relocated behavior was characterized before this adaptation and is rechecked afterwards. A valid one-page PDF retains its count; deeply nested hostile PDFs are now rejected without aborting the process. This intentional correction addresses [RUSTSEC-2026-0187](https://rustsec.org/advisories/RUSTSEC-2026-0187) and [RUSTSEC-2026-0204](https://rustsec.org/advisories/RUSTSEC-2026-0204). It is not runtime convergence, an Organizy dependency upgrade, or release acceptance.

## Read-only discovery enforcement

A pre-fix regression reproduced `git status` refreshing a tracked repository's index cache during catalog discovery. Discovery now sets `GIT_OPTIONAL_LOCKS=0`, which [Git documents](https://git-scm.com/docs/git#Documentation/git.txt-codeGITOPTIONALLOCKScode) as preventing that side effect. The tracked-index, configuration and worktree characterization tests verify this behavior. This is the explicit read-only migration constraint, not a project-discovery rewrite; worktree detection, Git-error reporting and marker rules retain their documented limitations. Unix inaccessible-directory behavior is characterized with actual denied access; Windows ACL qualification remains separate from the CLI build/integration matrix.
