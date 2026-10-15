# Attribution and licensing

Imported compatibility engine and Go wrapper source was authored by Arman Avanesyan. The owner authorized MIT licensing for imported owner-authored code. The wrapper's existing MIT copyright notice (2026 meshory) is retained verbatim in its LICENSE and LICENSE-MIT. The Go project catalog is also covered by LICENSE-MIT under the owner's explicit migration decision.

Scanner's pre-existing Apache-2.0 root LICENSE is retained. At migration, original Rust package metadata declared MIT despite that root text. On 2026-10-15 the owner explicitly authorized MIT OR Apache-2.0 for eligible original Rust code; original package metadata now declares that choice. LICENSE-MIT supplies the owner's MIT grant; imported compatibility components retain their MIT-only terms and existing notices. No dependency is relicensed: Rust crates and Go/Fyne dependencies retain their respective upstream licenses and notices. No vendored third-party source is introduced.

Original private repository commit IDs, branch/tag mappings, recovery bundles and asset inventories remain in private storage. Only reviewed source snapshots are public. Public project catalog history is retained as ancestry.

## Original Rust authorship review

The original `scanner/`, `scanfs/` and `crates/` history identifies Arman Avanesyan
under his personal and GitHub identities; automated maintenance commits are
attributed to github-actions. The owner grant applies to original project code,
not dependencies or embedded third-party material. Apache text and imported
wrapper copyright notices remain intact. No published package metadata, release
artifact or dependency license is changed retroactively.
