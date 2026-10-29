# Native dependency maintenance review — 2026-10-30

The [reviewed registry/source record](maintenance-review-2026-10-30.json)
refreshes the October 26 snapshot without adopting a dependency or extending a
policy disposition. The qualified runtime graph and production pins remain
unchanged. The native maintenance review cutoff stays **before 2026-11-24**.

| Component | Fresh finding | Decision |
| --- | --- | --- |
| [AccessKit Unix 0.24.1](https://crates.io/crates/accesskit_unix/0.24.1) | Still requires zbus `^5.19` | No published compatible zbus 6 path through this adapter |
| [AT-SPI common 0.21.1](https://crates.io/crates/accesskit_atspi_common/0.21.1) | Still requires zvariant `^5.15` | Do not force a transitive major version into this graph |
| [zbus 6.2.0](https://crates.io/crates/zbus/6.2.0) | Rust 1.89 required; Scanner is pinned to Rust 1.88 | A migration needs adapter/API integration and a separately reviewed toolchain decision |
| zvariant / derive / zbus_names | Latest reviewed versions 5.15.1 / 5.15.1 / 4.3.5; upstream notices still say unmaintained with no patched versions | Patch-number updates do not remove the notices; retain the reviewed locked graph and cutoff |
| [AccessKit macOS 0.27.2](https://crates.io/crates/accesskit_macos/0.27.2) | Changes SubclassingAdapter attachment, KVO/subclass detection and drop behavior; adds integration coverage | Source-reviewed candidate only. Review Scanner's post-Show attachment ordering and native lifecycle before adoption; retain qualified 0.27.1 |
| [AccessKit Windows 0.35.2](https://crates.io/crates/accesskit_windows/0.35.2) | Changes password-input recognition to the consumer helper; reviewed Rust delta does not alter shutdown code | Source-reviewed candidate only; not a demonstrated fix for the historical STA timeout. Retain qualified 0.35.1 |
| [Fyne v2.8.2](https://github.com/fyne-io/fyne/releases/tag/v2.8.2) | Latest release confirmed; Go module archive authenticated by sumdb. [Shutdown proposal #6442](https://github.com/fyne-io/fyne/pull/6442) is closed and unmerged | Retain the qualified local shutdown and Cocoa-wait adaptations; no toolkit switch |
| PNG / inout | Latest reviewed versions remain the already-qualified 0.18.1 / 0.2.5 | No new root upgrade justified by this review |

The two native patch archives match registry SHA-256 digests; Rust source
deltas were inspected. Their archives omit license text files. The upstream
MIT and Apache texts at the embedded source commit were verified against the
retained notices and match exactly. These are reviewed candidates,
not native qualification results. A patch-version label alone does not establish
attachment-order, password, event or shutdown compatibility.

The [three native maintenance dispositions](native-advisories.md) remain
informational unmaintained notices, not reported exploitable vulnerabilities.
Fresh advisory sources still contain no patched versions. The upstream
[zbus migration record](https://github.com/z-galaxy/zbus/issues/2043) is closed,
but that does not mean AccessKit's published zbus 5 integration has migrated.

[Follow-up #26](https://github.com/organizyio/scanner/issues/26) tracks the
compatible adapter migration before the existing cutoff. Every published zbus 6
version in this review (6.0.0, 6.1.0 and 6.2.0) declares Rust 1.89. The JSON
records selected native dependency edges, not a complete replacement lockfile.

A replacement must preserve license/source notices, Rust compatibility, cache
wire and negative controls, and native lifecycle/client evidence. Remove the
old dispositions only when the graph no longer contains the affected crates.
Do not add exceptions, extend the cutoff or substitute an incompatible lock.
This work is dependency maintenance, separate from scanning runtime convergence.
M1/static-2x evidence retains its tested revision; display transitions and
release acceptance remain separate.
