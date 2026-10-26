# Dependency review after PNG qualification

Review baseline: Scanner `82986118edf3c24fab0fa2deb1ef07b2957aa2f2`.
[Checksums, source findings and decisions](maintenance-png-review-2026-10-26.json)
retain fresh registry responses, authenticated archive digests and selected source
hashes. The [earlier review](maintenance-review-2026-10-26.md) remains historical;
its latest-publication findings are superseded here.

| Dependency | Verified finding | Decision |
| --- | --- | --- |
| [PNG 0.18.1](https://crates.io/crates/png/0.18.1) | MIT OR Apache-2.0, MSRV 1.73, non-yanked; reader accepts a decoder budget | Adopt this exact PNG reader with separate encoded, dimension and decoded-output checks. Keep existing image types, gradient algorithm and goldens. Remove the old PNG 0.16.8 graph |
| [inout 0.2.5](https://crates.io/crates/inout/0.2.5) | MIT OR Apache-2.0, MSRV 1.85; non-yanked replacement for locked 0.2.3. Published changes correct padding/output-length handling | Update only this root lock node. Test AES PDF stream padding boundaries through lopdf's in-memory public API, plus existing metadata/consumer regressions. This does not qualify new encrypted-PDF file support |
| [AccessKit Unix 0.24.1](https://crates.io/crates/accesskit_unix/0.24.1) | Published archive includes cache-signal fix #819; requires zbus 5.19 | Source-reviewed candidate, not adopted. Retain qualified vendor adaptation until its replacement passes cache wire controls and native replay |
| [AT-SPI common 0.21.1](https://crates.io/crates/accesskit_atspi_common/0.21.1) | Still requires zvariant 5.15 | Does not resolve the three maintenance notices; retain qualified native lock |
| zvariant / derive / zbus_names | Registry versions 5.15.1 / 5.15.1 / 4.3.5; fresh RustSec sources still report unmaintained with no patched version | No ineffective patch-number update or disposition extension |
| [Fyne v2.8.2](https://github.com/fyne-io/fyne/releases/tag/v2.8.2) | Latest release rechecked; authenticated archive, GLFW loop and channel hashes equal the previous source review. Proposal #6442 remains unmerged | Retain qualified shutdown adaptation; source review does not replace native qualification |
| image / img_hash | Latest image 0.25.10 is outside img_hash 3.2.0's image version range | Keep image 0.23.14 pixel/resizing types without legacy PNG codec features. A hash-stack migration requires separate compatibility review |

PNG and inout archives match registry SHA-256 digests, and their packaged
license texts were reviewed. Cargo-deny uses the unchanged root/native policies.
The root lock also adds PNG's fdeflate dependency and reuses already locked
flate2/CRC/modern miniz dependencies; arbitrary image codecs are not enabled.
Compatibility locks, native runtime code, Fyne pins and production consumer pins
are unchanged. Original import/provenance hashes and third-party notices remain.

The native informational review cutoff remains **before 2026-11-24**. No IDs,
version dispositions or deadlines are relaxed. AccessKit's newly published cache
fix is a useful replacement candidate, while the zbus 5 maintenance migration
remains unresolved. The newer native release requires its own integration and
host evidence; this change does not silently switch that graph.

Validation and exact-head checks are linked from [PNG hardening](png-decoding-limits.md).
Apple Silicon/Retina remains pending. Runtime convergence, production adoption,
release publication and release acceptance remain separate.
