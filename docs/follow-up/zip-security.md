# ZIP metadata dependency security maintenance

Both Rust metadata graphs pin reviewed `zip =9.0.5` (MIT, Rust 1.88). This
addresses [RUSTSEC-2026-0367](https://rustsec.org/advisories/RUSTSEC-2026-0367.html),
[RUSTSEC-2026-0368](https://rustsec.org/advisories/RUSTSEC-2026-0368.html) and
[RUSTSEC-2026-0369](https://rustsec.org/advisories/RUSTSEC-2026-0369.html).
Scanner reads `docProps/core.xml` through `ZipArchive::by_name` and never invokes
ZIP extraction APIs. The upgrade removes the vulnerable dependency rather than
adding advisory exceptions or changing the license policy.

The preferred graph retains its deflate feature. The compatibility graph
explicitly retains its previous archive formats; `bzip2-rs` selects the upstream
native C backend (`bzip2-sys`) in this ZIP version. Original MIT imports and
third-party notices remain intact. The native accessibility dependency graph and
its reviewed AccessKit backport are independent of these metadata graphs.

Regression tests in both metadata crates cover stored and deflated DOCX metadata,
Unicode and XML entities, unchanged revision strings, malformed/missing metadata,
and a traversal-named member that must never be written to disk. Validation uses
locked graphs, pinned Rust 1.88, the existing security/license policy, full Rust
checks and required real-engine wrapper integration on the component CI matrix.
Warnings already permitted by policy remain warnings; they are not support or
security acceptance claims.

The immutable consolidation baseline is retained. Reviewed current hashes in
`changes.json` account for this subsequent dependency and test change. Package
versions, public signatures, JSON fields and production consumer pins are
unchanged. Exact-main consumer qualification must be refreshed after this change
merges; earlier results remain evidence for their recorded revision only.

This is source security maintenance. It does not publish a crate or release,
adopt a production consumer dependency, converge runtimes, or complete pending
interactive macOS arm64/Retina qualification.
