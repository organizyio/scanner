# Scanner repository instructions

Sole owner: @ArmanAvanesyan. Keep root Scanner facade/API/schema and consumer pins stable during repository consolidation.

`compat/scan-engine` is an isolated Cargo workspace; `compat/scan-fs` and `tools/project-catalog` retain their original Go module identities. Do not add compatibility crates to root publishing. Build wrapper engines from local reviewed source with `scripts/build-components.py`; no private release downloads or `latest` resolution.

Never fetch private repository history, refs, tags or bundles into this public checkout. Private evidence belongs outside this repository. Preserve the public catalog's imported ancestry and record transformations in docs/consolidation.

Run root `make check`, isolated engine pinned Rust checks, wrapper unit plus required real-engine integration, project Go characterization, and `python3 scripts/verify-consolidation.py`. Native Fyne compilation does not prove GUI operation. Keep release acceptance separate.
