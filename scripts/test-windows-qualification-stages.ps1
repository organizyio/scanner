# Real cross-process regression: a blocked invocation cannot hide behind an exit wait.
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'windows-qualification-stages.ps1')
$directory = Join-Path ([IO.Path]::GetTempPath()) ('scanner-stage-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $directory | Out-Null
$worker = $null
try {
    $nonce = [guid]::NewGuid().ToString('N')
    if (Read-QualificationDeadline $directory $nonce) { throw 'An unpublished first stage was accepted' }
    $start = Write-QualificationStage $directory $nonce 'quit' $true
    if (Read-QualificationDeadline $directory $nonce) { throw 'Fresh stage expired' }
    Write-QualificationStage $directory $nonce 'quit' $false $start | Out-Null
    if (Read-QualificationDeadline $directory $nonce) { throw 'Completed stage expired' }
    $rejected = $false
    try { Read-QualificationDeadline $directory 'another-run' | Out-Null } catch { $rejected = $true }
    if (-not $rejected) { throw 'Foreign nonce accepted' }
    $stageScript = Join-Path $PSScriptRoot 'windows-qualification-stages.ps1'
    # Use a literal variable token, independent of the invoking script's scope.
    $code = ". '$stageScript'; Write-QualificationStage '$directory' '$nonce' 'quit' " + '$true' + ' 0 100 | Out-Null; Start-Sleep -Seconds 10'
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($code))
    # Retain the previous completed stage until the new writer publishes.
    $worker = Start-Process powershell.exe -ArgumentList @('-NoProfile','-NonInteractive','-EncodedCommand',$encoded) -PassThru
    $workerHandle = $worker.Handle
    $timer = [Diagnostics.Stopwatch]::StartNew()
    $expired = $null
    do { Start-Sleep -Milliseconds 10; $expired = Read-QualificationDeadline $directory $nonce }
    while (-not $expired -and $timer.ElapsedMilliseconds -lt 6000)
    if (-not $expired -or $expired.stage -ne 'quit' -or $worker.HasExited) { throw 'Blocked invocation was not detected while still running' }
    if ($expired.elapsed_ms -lt 100 -or $expired.elapsed_ms -gt 1500) { throw 'Deadline observation was not bounded' }
    # Process exit does not erase an already expired stage.
    Stop-Process -Id $worker.Id
    $worker.WaitForExit()
    if (-not (Read-QualificationDeadline $directory $nonce)) { throw 'Expired invocation was accepted after exit' }
    # Exercise actual File.Replace publication concurrently, including Windows'
    # transient missing-name state. No malformed/foreign record is tolerated.
    Write-QualificationStage $directory $nonce 'replace-before' $false | Out-Null
    $code = '$ErrorActionPreference="Stop"; ' + ". '$stageScript'; " + 'for ($i=0;$i -lt 200;$i++) { Write-QualificationStage ' + "'$directory' '$nonce' " + '("replace-"+$i) $false | Out-Null }'
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($code))
    $worker = Start-Process powershell.exe -ArgumentList @('-NoProfile','-NonInteractive','-EncodedCommand',$encoded) -PassThru
    $workerHandle = $worker.Handle
    $timer.Restart(); $reads=0
    while (-not $worker.WaitForExit(1) -and $timer.ElapsedMilliseconds -lt 10000) {
        if (Read-QualificationDeadline $directory $nonce) { throw 'Completed replacement stage became active' }
        $reads++
    }
    if (-not $worker.HasExited -or $worker.ExitCode -ne 0 -or $reads -lt 1) { throw 'Concurrent replacement writer/reader did not complete' }
    $published = Get-Content -LiteralPath (Join-Path $directory 'deadline.json') -Raw | ConvertFrom-Json
    if ($published.stage -ne 'replace-199' -or $published.run_id -ne $nonce) { throw 'Replacement stress did not publish every stage' }
    # Once observed, permanent disappearance is a bounded failure, not null.
    Remove-Item (Join-Path $directory 'deadline.json')
    $timer.Restart(); $rejected=$false
    try { Read-QualificationDeadline $directory $nonce | Out-Null } catch { $rejected=$true }
    if (-not $rejected -or $timer.ElapsedMilliseconds -gt 1500) { throw 'Missing observed stage was silently accepted' }
    # Real exclusive-handle contention must not produce a false completed stage
    # or extend an already expired monotonic deadline.
    $expiredStart = [Diagnostics.Stopwatch]::GetTimestamp() - 9 * [Diagnostics.Stopwatch]::Frequency
    Write-QualificationStage $directory $nonce 'replacement-contention' $true $expiredStart | Out-Null
    $ready = Join-Path $directory 'exclusive-ready'
    $path = Join-Path $directory 'deadline.json'
    $beginRead = Join-Path $directory 'begin-contended-read'
    $code = '$s=[IO.File]::Open(' + "'$path'," + '[IO.FileMode]::Open,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None); try { [IO.File]::WriteAllText(' + "'$ready','ready'); " + '$wait=[Diagnostics.Stopwatch]::StartNew(); while (-not [IO.File]::Exists(' + "'$beginRead'" + ') -and $wait.ElapsedMilliseconds -lt 6000) { Start-Sleep -Milliseconds 1 }; if (-not [IO.File]::Exists(' + "'$beginRead'" + ')) { throw "Reader never arrived" }; ' + ' } finally { $s.Dispose() }'
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($code))
    $worker = Start-Process powershell.exe -ArgumentList @('-NoProfile','-NonInteractive','-EncodedCommand',$encoded) -PassThru
    $workerHandle = $worker.Handle
    $timer.Restart()
    while (-not (Test-Path $ready) -and $timer.ElapsedMilliseconds -lt 6000) { Start-Sleep -Milliseconds 1 }
    if (-not (Test-Path $ready)) { throw 'Contending writer did not start' }
    $contended = $false
    try { $probe = [IO.File]::OpenRead($path); $probe.Dispose() }
    catch { if (($_.Exception.GetBaseException().HResult -band 0xffff) -eq 32) { $contended = $true } else { throw } }
    if (-not $contended) { throw 'Contention regression did not exercise a denied open' }
    # Keep the cross-process lock held while exercising bounded rejection.
    # Do not assume a child sleeping 100ms is rescheduled within the 200ms
    # reader budget on loaded CI hosts.
    $timer.Restart(); $rejected=$false
    try { Read-QualificationDeadline $directory $nonce | Out-Null }
    catch { $rejected=$true }
    if (-not $rejected -or $timer.ElapsedMilliseconds -gt 1500) { throw 'Cross-process contention did not fail within its bound' }
    [IO.File]::WriteAllText($beginRead, 'release')
    if (-not $worker.WaitForExit(5000) -or $worker.ExitCode -ne 0) { throw 'Contending writer did not release its handle' }
    $observed = Read-QualificationDeadline $directory $nonce
    if (-not $observed -or $observed.stage -ne 'replacement-contention' -or $observed.elapsed_ms -lt 9000) { throw 'Sharing failure hid or extended the original deadline' }
    # Persistent contention remains a bounded failure, never a null/success.
    $held = [IO.File]::Open($path,[IO.FileMode]::Open,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
    $timer.Restart(); $rejected = $false
    try { Read-QualificationDeadline $directory $nonce | Out-Null }
    catch { $rejected = $true }
    finally { $held.Dispose() }
    if (-not $rejected -or $timer.ElapsedMilliseconds -gt 1500) { throw 'Persistent sharing conflict was not rejected promptly' }
    [IO.File]::WriteAllText($path, '{malformed')
    $rejected = $false
    try { Read-QualificationDeadline $directory $nonce | Out-Null } catch { $rejected = $true }
    if (-not $rejected) { throw 'Malformed stage accepted' }
    [IO.File]::WriteAllText($path, (@{run_id=$nonce;active=$false} | ConvertTo-Json))
    $rejected=$false
    try { Read-QualificationDeadline $directory $nonce | Out-Null } catch { $rejected=$true }
    if (-not $rejected) { throw 'Incomplete completed-stage record accepted' }
    # Deterministically model exit between HasExited and StartTime reads.
    $raced = [pscustomobject]@{exited=$false}
    $raced | Add-Member ScriptProperty HasExited { $this.exited }
    $raced | Add-Member ScriptProperty StartTime { $this.exited=$true; return $null }
    if (Test-QualificationProcessIdentity $raced 1) { throw 'Exited process accepted as live' }
    $live = [pscustomobject]@{HasExited=$false;StartTime=[DateTime]::UtcNow}
    $ticks = $live.StartTime.ToUniversalTime().Ticks
    if (-not (Test-QualificationProcessIdentity $live $ticks)) { throw 'Live matching identity rejected' }
    if (Test-QualificationProcessIdentity $live ($ticks+1)) { throw 'Foreign process identity accepted' }
    $unknown = [pscustomobject]@{HasExited=$false;StartTime=$null}
    $rejected=$false
    try { Test-QualificationProcessIdentity $unknown 1 | Out-Null } catch { $rejected=$true }
    if (-not $rejected) { throw 'Unknown live process identity accepted' }
    # Exercise both actual PowerShell apartment modes, not a mocked flag.
    foreach ($mode in @('Mta','Sta')) {
        $call = if ($mode -eq 'Mta') {
            'Assert-QualificationAutomationThread | Out-Null'
        } else {
            'try { Assert-QualificationAutomationThread | Out-Null; exit 1 } catch { exit 0 }'
        }
        $code = '$ErrorActionPreference="Stop"; ' + ". '$stageScript'; " + $call
        $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($code))
        $worker = Start-Process powershell.exe -ArgumentList @('-NoProfile','-NonInteractive',('-'+$mode),'-EncodedCommand',$encoded) -PassThru
        $workerHandle = $worker.Handle
        if (-not $worker.WaitForExit(10000) -or $worker.ExitCode -ne 0) { throw ('UIA apartment contract failed: '+$mode) }
    }
    # Delayed readiness must be polled before one Invoke, never after it.
    $state = [pscustomobject]@{reads=0;calls=0}
    Invoke-QualificationEnabledControl -Resolve {
        $state.reads++
        [pscustomobject]@{Current=[pscustomobject]@{IsEnabled=($state.reads -ge 3)}}
    } -Invoke { param($control); if (-not $control.Current.IsEnabled) { throw 'Invoked a disabled control' }; $state.calls++ }
    if ($state.reads -ne 3 -or $state.calls -ne 1) { throw 'Native readiness did not precede exactly one action' }
    $state.calls=0; $rejected=$false
    try {
        Invoke-QualificationEnabledControl -Resolve { [pscustomobject]@{Current=[pscustomobject]@{IsEnabled=$true}} } -Invoke { param($control); $state.calls++; throw 'ambiguous-invoke' }
    } catch { $rejected=$_.Exception.Message -eq 'ambiguous-invoke' }
    if (-not $rejected -or $state.calls -ne 1) { throw 'Ambiguous native Invoke was retried or accepted' }
    $state.calls=0; $rejected=$false
    try {
        Invoke-QualificationEnabledControl -TimeoutMs 50 -Resolve { [pscustomobject]@{Current=[pscustomobject]@{IsEnabled=$false}} } -Invoke { param($control); $state.calls++ }
    } catch { $rejected=$true }
    if (-not $rejected -or $state.calls) { throw 'Permanently disabled control was invoked or accepted' }
    Write-Output 'Cross-process invocation, nonce, contention, persistent-lock and malformed-record gates passed'
} finally {
    if ($worker -and -not $worker.HasExited) { Stop-Process -Id $worker.Id }
    Remove-Item -LiteralPath $directory -Recurse -Force
}
