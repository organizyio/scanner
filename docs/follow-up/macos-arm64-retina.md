# Reviewed Apple Silicon and Retina qualification

Reviewed 2026-10-30. The [normalized results](macos-arm64-retina-results.json)
record external evidence for Scanner
`92100f46ce0bd1c93a0145df080874e62866b634`, the published merge of
[the macOS event-loop correction](https://github.com/organizyio/scanner/pull/24).
The [current qualification register](current-qualification.json) owns current
status; earlier reports retain their original revisions and failed observations.

## Recorded host and accepted scope

The external operator used a fresh workspace on MacBookPro17,1 (Apple M1),
macOS 27.0.1 (26A434), native arm64, with one built-in display reporting actual
2× backing scale. The returned source status is clean. Build records identify
native arm64 binaries and the same GUI binary hash across the flow, latency,
shutdown and pointer reports. This is recorded-host evidence, not independent
live access to that Mac or a claim about all macOS hardware.

| Gate | Reviewed result |
| --- | --- |
| Native arm64 build and checks | All 20 checks passed; no Rosetta execution reported |
| Serial AX latency | Four gate samples: median 0.017–0.021 ms per attribute, snapshots 15–31 ms, including Main plus Settings |
| Full native GUI flow | Unicode, folder-error recovery, profiles, tree keyboard access, pause/resume/cancel and settings/checkpoint restart passed |
| Shutdown | All 16 cases exited 0 without force; Quit 0.737–1.056 seconds |
| Settings-open shutdown | Attached, detached-before-Quit and no-observer cases passed; the original failing case is superseded within this rerun scope |
| Main actual 2× pointer | Real click on Add Folder opened the picker; Cancel returned to Main |
| Settings actual 2× pointer | Real click on Browse root 1 opened the picker; Cancel returned to Settings |
| Same-process 2×/1×/2× transitions | Pending: no physical 1× display available |
| Broader spoken-reader usability | Unqualified; additional spoken testing deferred |

Both pointer captures use the same manual-session process, scale 2, window
fraction 1.0, foreground ownership and matching center hit tests. Main and
Settings are independently checked. Static 2× operation does not qualify
cross-display transitions. POSIX advisory-lock observations do not qualify
Windows mandatory-sharing denial. Across all 94 latency files, the worst median
was approximately 0.116 ms and the worst snapshot 150.1 ms; these measurements
are observations on this run, not worst-case performance guarantees.

## Review and provenance

The external checksum and all 827 archive members, including 52 PNG images, verified
against the locally retained trusted rerun profile. That profile pins the
reviewed source and helper hashes. The verifier confirmed source/build identity,
all named check logs, canonical GUI/shutdown reports and both Retina pointer
reports. No failure or forced-cleanup files were found.

Every screenshot was inspected in contact-sheet overview; seven full-size
samples covered folder errors, tree focus, Unicode and Main/Settings pointer
states. The operator separately reports reviewing all 52 screenshots. Window
captures and resulting picker states support the reported application behavior.
An unrelated system prompt was reported untouched; reported click centers lie
outside its stated bounds. Application-window-only captures cannot independently
prove the entire desktop was unobscured.

The verifier's aggregate result remains `needs_review`: display transitions are
pending, and passed freeform notes about unexpected behavior, privacy and prior
run retention have empty evidence-reference lists. This is evidence bookkeeping,
not an additional observed application failure. Scoped passes above follow
report/image review; the original operator and verifier outputs were not edited
to manufacture an aggregate pass. The operator's claim that the previous remote
workspace remained unchanged is retained as an attributed claim.

Raw archives, screenshots, personal paths, logs and private recovery material
remain outside public Git. The JSON publishes normalized observations and
checksums only. A limited credential-pattern scan found no matches; it does not
certify the absence of every kind of private content.

## Remaining boundaries

Long paths can be clipped without ellipsis or wrap midword, and Settings fields
and the Browse root 1 button have tight right-edge spacing. These layout
follow-ups remain documented; they do not become release acceptance through
this qualification update.

The [earlier partial M1 run](macos-ax-latency.md) and its source evidence remain
historical. The new evidence closes native arm64 and static Main/Settings 2×
gates at the tested revision. Phase 4 stays incomplete for display transitions
and deferred broader reader gates. Production consumer pins are unchanged.
Runtime convergence, release publication and release acceptance remain separate.
