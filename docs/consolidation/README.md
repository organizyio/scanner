# Scanning repository consolidation

Owner-approved execution packet, 2026-10-15. Sole owner: @ArmanAvanesyan.
This record defines the migration boundary and baseline. Original repositories remain authoritative until cutover gates pass; [migration PR #3](https://github.com/organizyio/scanner/pull/3) records the validated revisions, consumer acceptance, immutable artifact tags and archival outcome.
Repository consolidation does not establish runtime convergence or product release acceptance.

## Repository layout and disposition

| Component | Destination | Preservation |
| --- | --- | --- |
| Scanner facade and `scanfs` | Existing root workspace | API, package identity and consumer pins unchanged |
| Go project catalog, CLI and Fyne | `tools/project-catalog` | Public original commits retained as ancestors; exact initial tree recorded |
| Earlier engine | `compat/scan-engine` | Reviewed source-only snapshot; private history and recovery remain private |
| Go engine wrapper | `compat/scan-fs` | Reviewed source-only snapshot; original module identity and MIT notice retained |

See [manifest](manifest.json), [compatibility](compatibility.md), [validation and cutover](validation.md) and [licensing notices](NOTICE.md).

Public catalog baseline: `a51075a6c6f389bc906a2f71ede75cc2c00eb04d` from [scan-projects](https://github.com/organizyio/scan-projects). The non-squashed import precedes all transformations. Private refs, exact source/tree hashes, file reviews, bundle hashes and original release assets are recorded separately in private evidence. No private commits, refs, tags or bundles are imported into this repository or its releases. Historical PRs, issues, discussions and immutable releases stay in their original repositories.

## Build and distribution

The separately reviewed [security prerequisite](https://github.com/organizyio/scanner/pull/2) qualifies parser/dependency patches without changing consumer pins. The root `make check` validates the preferred Rust implementation. Compatibility workspaces are isolated and cannot publish crates. From the repository root:

```sh
python3 scripts/build-components.py engine --archive
python3 scripts/build-components.py wrapper --archive
python3 scripts/build-components.py catalog --archive
python3 scripts/build-components.py launchers --archive
python3 scripts/verify-consolidation.py
```

Wrapper release hosts are Linux x64, Windows x64 and macOS arm64. Each build compiles the reviewed local engine and embeds its target-matched payload. No build downloads a private release or selects `latest`. Intel macOS retains the external-engine-only wrapper contract using `SCAN_ENGINE_PATH`.

Archives have component-qualified names, include licenses and have SHA-256 sidecars. The `engine/vX.Y.Z`, `wrapper/vX.Y.Z`, `catalog/vX.Y.Z` and `launchers/vX.Y.Z` tags are independent from Scanner's existing `v*` releases. The explicit component-release workflow accepts only successful exact-main component validation artifacts. It never publishes crates or overwrites a release. Native Fyne compilation runs in CI; GUI operational qualification remains a separate requirement before GUI distribution/support claims.

Do not install both `scan-fs` implementations into the same binary directory. Legacy launcher archives contain only launchers; place them alongside the intended catalog commands, including a separately qualified `scan-gui` if using the `scanner` launcher. They never redirect into the wrapper archive.

## Consumer boundaries

Organizy keeps its published Scanner pin and managed traversal, root identity fencing, authorization and durable delivery. This repository move changes no runtime API/schema. Organizy Scan's substrate check must use this repository's compatibility engine at the same exact checkout revision. Archivist has an independent embedded engine and must be reviewed separately; it is not replaced here.

A Rust project-discovery port, marker/Git fixes, bounded live callbacks, supported Go SDK and dependency upgrades are separate follow-ups. Historical repository-intelligence proposals are not accepted requirements by import.
