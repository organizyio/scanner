# Dependency maintenance review — 2026-10-26

Reviewed against Scanner `1e27d747799b852da0e0b3e73987fc4d5599715e`.
[Source checksums and decisions](maintenance-review-2026-10-26.json) record
published archives, inspected source files and unchanged dependency/policy files.
This review adopts no dependency and extends no exception deadline.

| Component | Verified publication/source | Decision |
| --- | --- | --- |
| AccessKit Unix | [Published 0.24.0](https://crates.io/crates/accesskit_unix/0.24.0) is still latest; its archive checksum equals our original unpatched import. [Cache fix #819](https://github.com/AccessKit/accesskit/pull/819) is merged, but not in that release | Retain the reviewed cache-signal backport |
| AccessKit AT-SPI common | [Published 0.21.0](https://crates.io/crates/accesskit_atspi_common/0.21.0) is still latest; published Unix/common manifests retain zbus 5 / zvariant 5 | Keep the qualified native graph; a compatible upstream zbus 6 migration remains follow-up |
| zvariant / derive / zbus_names | Latest published versions are 5.15.1 / 5.15.1 / 4.3.5; their package descriptions remain deprecated/merged into zbus. All three [RustSec maintenance notices](native-advisories.md) still have no patched version | Do not treat a patch-number update as resolving unmaintained notices. Retain qualified lock versions 5.15.0 / 5.15.0 / 4.3.4 and the exact existing dispositions |
| Fyne | [Published releases](https://github.com/fyne-io/fyne/releases) include v2.7.4 and latest v2.8.2. Both checksum-authenticated module archives retain the queued stopped hook, raw channel submission and output-length drain before channel close. [Proposal #6442](https://github.com/fyne-io/fyne/pull/6442) is closed without merge | Neither inspected release contains both required shutdown corrections; retain the reviewed v2.7.3 adaptation. New releases were source-reviewed, not native-qualified or adopted |

The Fyne source comparison examines the GLFW shutdown loop and its unbounded
channel, not only release notes or version numbers. The retained local queue
atomically seals submissions and acknowledges every accepted waiter through
end-of-stream; the published loops still drain by currently visible output
length. No claim is made that the newer versions were reproduced failing on a
native host.

The existing native-only advisory policy still expires **before 2026-11-24**.
The current review does not reset that date, add exemptions, loosen checks or
approve exploitable vulnerabilities. Root and compatibility policy remain
unchanged. Run the existing policy checker and native cargo-deny validation.

For eventual adaptation retirement, identify a published fixed archive and
checksum, review MSRV/license/source changes, run cache wire positive/negative
controls and queue/lifecycle regressions, then replay available native hosts.
Missing Apple Silicon/Retina evidence remains pending. Keep rollback artifacts
and original import/provenance hashes unchanged; native support, production
adoption, runtime convergence and release acceptance remain separate.
