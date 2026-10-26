# Real cross-process regression: a blocked invocation cannot hide behind an exit wait.
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'windows-qualification-stages.ps1')
$directory = Join-Path ([IO.Path]::GetTempPath()) ('scanner-stage-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $directory | Out-Null
$worker = $null
try {
    $nonce = [guid]::NewGuid().ToString('N')
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
    Remove-Item (Join-Path $directory 'deadline.json')
    $worker = Start-Process powershell.exe -ArgumentList @('-NoProfile','-NonInteractive','-EncodedCommand',$encoded) -PassThru
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
    # Real exclusive-handle contention must not produce a false completed stage
    # or extend an already expired monotonic deadline.
    $expiredStart = [Diagnostics.Stopwatch]::GetTimestamp() - 9 * [Diagnostics.Stopwatch]::Frequency
    Write-QualificationStage $directory $nonce 'replacement-contention' $true $expiredStart | Out-Null
    $ready = Join-Path $directory 'exclusive-ready'
    $path = Join-Path $directory 'deadline.json'
    $beginRead = Join-Path $directory 'begin-contended-read'
    $code = '$s=[IO.File]::Open(' + "'$path'," + '[IO.FileMode]::Open,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None); try { [IO.File]::WriteAllText(' + "'$ready','ready'); " + '$wait=[Diagnostics.Stopwatch]::StartNew(); while (-not [IO.File]::Exists(' + "'$beginRead'" + ') -and $wait.ElapsedMilliseconds -lt 6000) { Start-Sleep -Milliseconds 1 }; if (-not [IO.File]::Exists(' + "'$beginRead'" + ')) { throw "Reader never arrived" }; Start-Sleep -Milliseconds 100' + ' } finally { $s.Dispose() }'
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($code))
    $worker = Start-Process powershell.exe -ArgumentList @('-NoProfile','-NonInteractive','-EncodedCommand',$encoded) -PassThru
    $timer.Restart()
    while (-not (Test-Path $ready) -and $timer.ElapsedMilliseconds -lt 6000) { Start-Sleep -Milliseconds 1 }
    if (-not (Test-Path $ready)) { throw 'Contending writer did not start' }
    $contended = $false
    try { $probe = [IO.File]::OpenRead($path); $probe.Dispose() }
    catch { if (($_.Exception.GetBaseException().HResult -band 0xffff) -eq 32) { $contended = $true } else { throw } }
    if (-not $contended) { throw 'Contention regression did not exercise a denied open' }
    [IO.File]::WriteAllText($beginRead, 'start')
    $observed = Read-QualificationDeadline $directory $nonce
    if (-not $observed -or $observed.stage -ne 'replacement-contention' -or $observed.elapsed_ms -lt 9000) { throw 'Sharing retry hid or extended the deadline' }
    if (-not $worker.WaitForExit(5000)) { throw 'Contending writer did not release its handle' }
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
    Write-Output 'Cross-process invocation, nonce, contention, persistent-lock and malformed-record gates passed'
} finally {
    if ($worker -and -not $worker.HasExited) { Stop-Process -Id $worker.Id }
    Remove-Item -LiteralPath $directory -Recurse -Force
}
