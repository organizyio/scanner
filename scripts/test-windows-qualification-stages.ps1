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
    Write-Output 'Cross-process blocked-invocation, completed-stage and nonce deadline gates passed'
} finally {
    if ($worker -and -not $worker.HasExited) { Stop-Process -Id $worker.Id }
    Remove-Item -LiteralPath $directory -Recurse -Force
}
