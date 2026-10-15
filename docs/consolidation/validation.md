# Validation, cutover and rollback

This checklist defines acceptance. [Migration PR #3](https://github.com/organizyio/scanner/pull/3) records actual execution evidence. A merged repository change alone does not satisfy these gates.

## Required checks

1. Restore every source ref from verified private all-ref bundles and preserve release assets/checksums separately. Recheck remote refs, releases, open work, settings and access immediately before cutover.
2. Verify public catalog baseline ancestry and initial exact subtree hash with `scripts/verify-consolidation.py`. Review every imported source file, authorship, license and confidential-content disposition. Scan staged snapshots and public history for secrets.
3. Compare all public reachable commits/refs/objects against the private source history in private tooling; no private commit may occur. Repeat on an independent public remote clone before cutover. Public bundles/assets must contain no private Git storage or recovery metadata.
4. Run root pinned Rust checks. On Linux x64, Windows x64 and macOS arm64 build the local compatibility engine and embed it, run Go unit and mandatory real-engine integration tests, and build project CLI for Linux/macOS/Windows x64 and arm64. Missing engines must fail integration.
5. Compare original and relocated CLIs on identical fixtures: help/invalid arguments, defaults, flags/environment, output modes, hashes, timestamps, EXIF extras, omitted/null fields, large integers, Unicode/path normalization and interruption. Normalize only nondeterministic timestamps and ordering; record divergences rather than concealing them.
6. Run project characterization including nested Git/marker discovery, non-Git, worktree limitation, exclusions, Git errors/missing Git, inaccessible paths, profiles, hashing controls, aggregates, pause/resume, cancellation and checkpoint recovery. Native Fyne build is not GUI operation proof: qualify GUI on native graphical hosts before claiming support/distributing GUI.
7. Validate owning consumers at exact Scanner heads. Keep Organizy's published dependency pin and managed runtime unchanged. Independently review Archivist's embedded engine.
8. Verify owner/access, release gate, unchanged protections, root staged-file boundaries and exact original Organizy HEAD/status/binary diff. Retire source folders and worktrees only after separately reconciling unpublished work and verifying private recovery.

## Cutover order

Merge the release gate before any import. Merge consolidation only after its exact-head checks pass. Merge focused consumer changes against the merged Scanner revision and validate those exact heads. Publish component-qualified artifacts/checksums through the explicit release workflow. Recheck source heads/open work; add replacement notices, then archive the three originals. Private originals stay private; historical tags/releases remain immutable. Retain recovery bundles and original artifacts. Source directories may be retired after separate recovery verification.

If any gate fails, originals remain authoritative and archival stays pending. Roll back consumers to prior pins/artifacts; unarchive originals and explicitly restore authority if needed. Never force-push, rewrite history, delete original releases or replace published artifacts. Product release acceptance remains separate.
