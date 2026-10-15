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

- Scanner and the earlier engine enqueue records without a bound during synchronous traversal and consume them afterwards. Callback progress/cancellation is not bounded live streaming. This move makes no throughput, memory or cancellation improvement claim.
- Catalog Git discovery recognizes `.git` directories, not `.git` files used by worktrees/submodules.
- Failed Git status commands can report clean (`dirty=false`); missing Git does not establish a clean repository. Failed lookups can panic if callers supply a typed nil catalog logger; existing CLI supplies a logger.
- Catalog and deep scan have inconsistent stack markers (notably Go/Rust/Java). Primary stack is alphabetic, not measured dominance.
- Some platform creation times in the Go catalog fall back to modification times. They are not reliable birth-time evidence.
- Legacy traversal skips inaccessible entries and can return success/partial results. Symlink, exclusion and error semantics are not replaced with managed Organizy traversal.
- Git subprocesses are read-only but lack timeouts; remote URL fields retain their original raw values. Treat exported local inventory as potentially sensitive when remotes contain credentials.
- Resume is lexical and inclusive of the checkpoint path. It is not a filesystem snapshot or transactional restart. Deep scan retains collected results in memory.

Corrections and convergence need independent specs, tests and consumer qualification.

The wrapper currently can exit 1 when interrupted while its child is blocked, despite a separate code path for 130. Original and relocated binaries both returned 1 in the characterized blocked-child case. No stronger interruption guarantee is claimed.
