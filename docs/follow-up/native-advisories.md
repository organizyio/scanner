# Native Linux dependency maintenance dispositions

Reviewed 2026-10-24; [latest publication/source refresh](maintenance-png-review-2026-10-26.md) retains the same scope and deadline. Re-review **before 2026-11-24** and before any release.

RustSec marks [zvariant](https://rustsec.org/advisories/RUSTSEC-2026-0370.html),
[zbus_names](https://rustsec.org/advisories/RUSTSEC-2026-0371.html) and
[zvariant_derive](https://rustsec.org/advisories/RUSTSEC-2026-0372.html) as
informational **unmaintained**, with no patched versions. Upstream merged their
APIs into zbus 6 ([upstream migration record](https://github.com/z-galaxy/zbus/issues/2043)). These are maintenance notices, not reported exploitable
vulnerabilities. They still require a tracked migration.

Published [`accesskit_unix 0.24.1`](https://crates.io/crates/accesskit_unix/0.24.1)
and [`accesskit_atspi_common 0.21.1`](https://crates.io/crates/accesskit_atspi_common/0.21.1)
retain zbus 5 / zvariant 5 types. The new Unix archive includes the upstream
cache-signal fix #819, superseding the earlier latest-publication observation.
[The replacement qualification](accesskit-metadata-qualification.md) records
source/license review, private-bus positive/negative controls and native replay.
The runtime vendor adaptation is retired in favor of the published crate; the
separate Fyne adaptation remains. This does not resolve the three maintenance
notices. Forcing zbus 6 or replacing only one transitive package is not a
compatible lock refresh. Keep the advisory versions, review cutoff and hardware
qualification boundaries unchanged.

The native-only `deny.toml` adds precisely these three informational IDs. Root
and compatibility policies remain unchanged; ZIP vulnerability advisories are
resolved by upgrading, not exempted. The required CI checker rejects policy
relaxations, added/duplicate dispositions, changes to the reviewed dependency
versions (`zvariant 5.15.0`, `zvariant_derive 5.15.0`, `zbus_names 4.3.4`), and an
expired review window. Cargo-deny still checks vulnerabilities, licenses,
sources and all other advisories on the native graph.

Recheck upstream releases by the review date, on any native dependency change,
and before a release. A replacement needs a reviewed source/license delta,
Rust 1.88 qualification, cache wire/negative-control tests and native host
replay. Remove these dispositions when the graph no longer contains the old
crates. Keep unavailable macOS hosts pending; do not silently broaden a support
claim or adopt a production Scanner pin.
