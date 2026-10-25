# Returned Apple Silicon / Retina evidence inspection

The offline verifier inspects the existing handoff package format for reviewed
Scanner `c16955e76e4768c74d77fa3b4b3ac1a80c0af58d`. Its catalog/native/toolkit
source matches the later qualified application. The two trusted handoff helper
SHA-256 values are pinned in the verifier; a revised kit requires a separate
reviewed profile update. Do not repin it to make an unexpected package pass.

Run with Python 3.11 or newer, keeping the archive and report outside public Git:

```sh
python3 scripts/verify-returned-evidence.py /private/evidence/results.zip \
  --report /private/evidence/new-review.json
```

If a digest arrived through a trusted separate channel, supply
`--archive-sha256 FULL_SHA256`. The internal manifest alone establishes byte
consistency, not sender authenticity. The tool never extracts, executes, imports
or fetches package content. It creates a new report and refuses to overwrite one.

## Integrity and coverage

- Preflight the ZIP directory before allocation: at most 10,000 members, 8 MiB central directory, 64 MiB per member, 512 MiB archive/expanded total and 4 MiB per JSON file and 32 MiB aggregate JSON. Reject excessive compression ratios, split/ZIP64/encrypted archives and unsupported compression. These are inspection limits, not permission to truncate evidence; an oversize legitimate return needs a separately reviewed handling change.
- Reject traversal, absolute/drive/backslash paths, control characters, noncanonical Unicode, reserved device names, duplicate/case-colliding members, links/special files, private Git/SSH storage and recovery/executable artifacts. Hash each bounded byte stream and require exact manifest membership and hashes.
- Compare the trusted helper hashes, source status/revision, host architecture, declared binary hashes, summary and canonical reports. Require every named build/CLI check and its journal/log reference.
- Inventory native flow and sixteen shutdown scenarios, exit/deadline/forced-cleanup results, screenshot checksums and PNG container/CRC/dimensions. Pixel readability, authenticity and actual behavior remain manual review; PNG inspection does not decode or judge image content.
- Cross-check window scale/geometry/pointer claims with indexed screenshots, session identity and natural exit. The shipped macOS capture includes shadows; bounded equal pixel padding is allowed, and must be reviewed visually rather than used to infer backing scale. Same-process 2x → 1x → 2x evidence is separate from main/Settings Retina evidence. Missing display hardware stays missing rather than borrowing another session's PID.
- Retain operator failed/pending/not-run observations as incomplete coverage. Do not infer human visual/keyboard or privacy acceptance from a self-reported `passed` field.

Exit 2 means invalid archive/integrity. Exit 3 means intact but incomplete or
inconsistent evidence. Exit 0 means structurally consistent evidence is ready
for human review; it is **not** hardware/platform or release acceptance. Review
coverage items independently: unavailable display transitions need not erase
other useful evidence. A `ready_for_human_review` report always leaves formal
qualification and release acceptance false.

Private output includes only normalized coverage, not copied operator notes,
command lines or screenshot pixels. Inspect raw returns privately; integrity verification is not privacy approval. Test fixtures
are synthetic and never establish Apple Silicon/Retina operation. Original
handoff files and qualification evidence remain unchanged.
