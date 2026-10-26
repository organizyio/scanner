# Repeated native shutdown observations. Preserve every failure; do not retry in place.
param(
    [Parameter(Mandatory=$true)][string]$Binary,
    [Parameter(Mandatory=$true)][string]$Directory,
    [Parameter(Mandatory=$true)][ValidatePattern('^[a-f0-9]{64}$')][string]$ExpectedBinarySha256,
    [Parameter(Mandatory=$true)][ValidatePattern('^[a-f0-9]{40}$')][string]$SourceRevision,
    [string]$DebuggerPath=''
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'windows-qualification-stages.ps1')
$Binary = (Resolve-Path -LiteralPath $Binary).Path
if (Test-Path -LiteralPath $Directory) { throw 'Shutdown matrix requires a new evidence directory' }
New-Item -ItemType Directory -Path $Directory | Out-Null
$Directory = (Resolve-Path -LiteralPath $Directory).Path
$cases = @(
    @{name='detached'; count=10; mode='detach-before-quit'; actions='announcements,quit'; cycles=0},
    @{name='attached'; count=5; mode='attached'; actions='announcements,quit'; cycles=0},
    @{name='shortcut'; count=5; mode='detach-before-quit'; actions='announcements,quitshortcut'; cycles=0},
    @{name='no-observer'; count=5; mode='none'; actions='quit'; cycles=0},
    @{name='reattach'; count=1; mode='attached'; actions='announcements,quit'; cycles=10}
)
$results = @()
$runner = $null
$fixture = $null
$passed = $false
try {
    foreach ($case in $cases) {
        for ($index = 1; $index -le $case.count; $index++) {
            if ((Get-FileHash -Algorithm SHA256 -LiteralPath $Binary).Hash.ToLowerInvariant() -ne $ExpectedBinarySha256) { throw 'Qualification binary changed' }
            $fixture = Join-Path $Directory ($case.name + '-' + $index)
            & python (Join-Path $PSScriptRoot 'platform-qualification.py') prepare --fixture $fixture --files 6 > (Join-Path $Directory 'fixture-prepare.log')
            if ($LASTEXITCODE -ne 0) { throw 'Fixture preparation failed' }
            $arguments = '-NoProfile -NonInteractive -File "' + (Join-Path $PSScriptRoot 'platform-windows.ps1') + '" -Binary "' + $Binary + '" -Fixture "' + $fixture + '" -Actions ' + $case.actions + ' -ObserverMode ' + $case.mode + ' -ObserverCycles ' + $case.cycles
            if ($DebuggerPath) { $arguments += ' -DebuggerPath "' + (Resolve-Path -LiteralPath $DebuggerPath).Path + '"' }
            $runner = Start-Process powershell.exe -ArgumentList $arguments -PassThru -RedirectStandardOutput (Join-Path $fixture 'runner-stdout.txt') -RedirectStandardError (Join-Path $fixture 'runner-stderr.txt')
            $runnerHandle = $runner.Handle
            $end = (Get-Date).AddSeconds(180)
            while (-not $runner.WaitForExit(1000) -and (Get-Date) -lt $end) { }
            if (-not $runner.HasExited) { throw 'Qualification supervisor exceeded its outer bound' }
            if ($null -eq $runner.ExitCode -or $runner.ExitCode -ne 0) { throw ('Qualification runner failed: ' + $case.name + '-' + $index) }
            $out = Join-Path $fixture 'windows-ui'
            $report = Get-Content -LiteralPath (Join-Path $out 'report.json') -Raw | ConvertFrom-Json
            if ($report.automation_apartment -ne 'MTA' -or $report.automation_thread -le 0 -or ($case.mode -ne 'none' -and $report.event_registration_thread -ne $report.automation_thread)) { throw 'UIA client apartment/thread evidence missing' }
            $quit = $report.qualified.quit
            if ($report.error -or -not $quit -or $quit.exit_code -ne 0 -or $quit.forced -or $quit.invoke_error -or $quit.elapsed_ms -ge 8000 -or $quit.observer_mode -ne $case.mode) { throw 'Native Quit acceptance failed' }
            if (Test-Path -LiteralPath (Join-Path $out 'deadline-failure.json')) { throw 'Supervisor deadline failure cannot be qualified' }
            if ($case.mode -ne 'none' -and (-not $report.qualified.announcements.finished_event -or -not $report.qualified.announcements.affected_folder_error_event)) { throw 'Native observer events not verified' }
            if ($case.cycles -and ($report.qualified.observer_cycles.count -ne 10 -or -not $report.qualified.observer_cycles.reattached_events_verified)) { throw 'Observer reattachment did not preserve events' }
            $owned = Get-Content -LiteralPath (Join-Path $out 'process.json') -Raw | ConvertFrom-Json
            $left = Get-Process -Id $owned.pid -ErrorAction SilentlyContinue
            if ((Test-QualificationProcessIdentity $left $owned.start_ticks) -and $left.Path -eq $Binary) { throw 'Owned GUI process remains after native Quit' }
            if (Get-ScheduledTask -ErrorAction Stop | Where-Object { $_.TaskName -eq ('ScannerQualification-' + $owned.run_id) }) { throw 'Owned interactive task remains after qualification' }
            $timings = @(Get-Content (Join-Path $out 'stage-timings.jsonl') | ForEach-Object { $_ | ConvertFrom-Json })
            $results += @{case=$case.name; iteration=$index; run_id=$owned.run_id; quit=$quit; stages=$timings; observer_cycles=$case.cycles}
            Write-Output ($case.name + '-' + $index + ': passed; quit_ms=' + $quit.elapsed_ms)
        }
    }
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $Binary).Hash.ToLowerInvariant() -ne $ExpectedBinarySha256) { throw 'Qualification binary changed' }
    $passed = $true
} finally {
    if ($runner -and -not $runner.HasExited) { Stop-Process -Id $runner.Id }
    if ($fixture -and (Test-Path (Join-Path $fixture 'windows-ui/process.json'))) {
        $owned = Get-Content (Join-Path $fixture 'windows-ui/process.json') -Raw | ConvertFrom-Json
        $task = 'ScannerQualification-' + $owned.run_id
        Stop-ScheduledTask -TaskName $task -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $task -Confirm:$false -ErrorAction SilentlyContinue
        $left = Get-Process -Id $owned.pid -ErrorAction SilentlyContinue
        if ((Test-QualificationProcessIdentity $left $owned.start_ticks) -and $left.Path -eq $Binary) { Stop-Process -Id $left.Id }
    }
    @{source_revision=$SourceRevision; binary_sha256=$ExpectedBinarySha256;
      matrix_passed=$passed; observations=$results; release_acceptance=$false;
      prior_unreproduced_failure='Retain pending unless its cause is demonstrated and corrected'} |
        ConvertTo-Json -Depth 9 | Set-Content -Encoding UTF8 (Join-Path $Directory 'shutdown-matrix.json')
}
