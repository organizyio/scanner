# Catalog path layout and discovery boundary

The implementation at `a96410b3d45517a01688e265cf8664f613933bfd` addresses the
layout observations from the returned M1 evidence without changing native adapter
versions, lifecycle code, control names, settings/checkpoints or JSON shapes.

Folder rows show their leaf name and enabled state before the parent path, with
visual ellipsis when needed. Native tree names and stable IDs retain their full
original paths. Selecting a row still exposes the complete path in the details
pane. Details and picker status scroll horizontally instead of wrapping inside a
path component. Settings has outer padding, including to the right of Browse.
Long text can still need scrolling; this does not promise that every path fits
within every window width.

Available Linux qualification exposed a separate catalog boundary defect: when
the selected root itself contains a marker, its ancestor search could escape
above that root. An unrelated ancestor project marker could suppress the selected
project. Two synthetic regressions, using `package.json` and `unrelated.py`
ancestor markers, fail before the correction and pass afterward. Ancestor
inspection now stops at the explicit root. Existing descendant discovery,
exclusions and marker rules remain; no Git mutation is introduced. An explicit
root can now produce a project row where the previous defect produced none.
This is a documented observable correction, not a schema change or convergence.

[Normalized qualification evidence](catalog-layout-2026-10-30.json) distinguishes
source-checked native builds, regression checks, actual GUI operation and retained
historical qualification. Failed and aborted attempts remain in private evidence.
No result is rewritten into a pass. The original M1/static-2x qualification stays
attached to `92100f46ce0bd1c93a0145df080874e62866b634`; it is not relabeled as a
physical test of the later layout. Same-process display transitions remain
pending. Broader spoken readers, production adoption and release acceptance
remain separate.

The [Windows UIA client review](windows-uia-client-review-2026-10-30.md) records
the legacy-client limitation independently of current MTA qualification.
Preferred Scanner and compatibility consumer trees remain unchanged; the
[reconciled consumer evidence](consumer-evidence-2026-10-30.md) therefore remains
applicable to those recorded paths without a production pin upgrade.

[PR #28 completion](https://github.com/organizyio/scanner/pull/28) records
the additional exact-source Windows shutdown observations and final validation.
The normalized snapshot above asserts its completed flow only; supplementary
runs do not qualify broader STA clients or physical display transitions.
