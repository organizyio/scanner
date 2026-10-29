# Windows UIA client contract review — 2026-10-30

[The diagnostic record](windows-uia-client-review-2026-10-30.json) distinguishes
the qualified MTA client from the legacy STA experiment. The binary, source,
client helpers and per-case report hashes are explicit; raw evidence remains
private. The original failed reports are unchanged.

Six controlled fresh-process cases completed with exit zero and no invocation
errors: MTA and diagnostic STA, each with observer attached, detached and absent.
Quit took 201–238 ms. The private STA adaptation changes client apartment checks
and diagnostic labeling only; it is excluded from acceptance. Three unchanged
legacy-client processes also exited naturally in 179–228 ms. Its fourth process
failed with a UIA invocation error before a recorded Quit stage. The series
stopped there. It did **not** reproduce the historical eight-second Quit timeout,
and no timeout stacks were produced. These observations establish neither a
provider deadlock nor a provider fix; successful diagnostic cases do not erase
failed cases.

The supported qualification harness continues to follow
[Microsoft's UIA threading guidance](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-threading):
MTA automation, with event subscription and removal on the same registration
thread. Real PowerShell MTA/STA regression processes verify that STA is rejected
before the app launches. The runtime adapter and its qualified dependency remain
unchanged by this investigation. The latest AccessKit Windows patch changes
password-input recognition, not the reviewed shutdown path; it is not adopted as
a speculative remedy.

[Issue #23](https://github.com/organizyio/scanner/issues/23) remains the explicit
diagnostic follow-up for broader clients. Legacy STA shutdown, arbitrary UIA
clients and spoken readers are unqualified. Reopening those support claims needs
an exact reproducer, owned-process diagnostics and a complete fresh client
matrix. Current MTA evidence cannot substitute for them. Display transitions,
runtime convergence and release acceptance remain separate.
