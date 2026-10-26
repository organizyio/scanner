# Shared native qualification stage protocol. Stopwatch ticks are monotonic and
# share the Windows performance-counter epoch across the supervisor and runner.
function Write-QualificationStage {
    param([string]$Directory, [string]$RunId, [string]$Name, [bool]$Active,
          [long]$StartedTicks = 0, [ValidateRange(1,8000)][int]$LimitMs = 8000)
    if (-not $StartedTicks) { $StartedTicks = [Diagnostics.Stopwatch]::GetTimestamp() }
    $self = Get-Process -Id $PID
    $record = @{run_id=$RunId; stage=$Name; active=$Active; started_ticks=$StartedTicks;
        frequency=[Diagnostics.Stopwatch]::Frequency; limit_ms=$LimitMs;
        runner_pid=$PID; runner_start_ticks=$self.StartTime.ToUniversalTime().Ticks}
    $path = Join-Path $Directory 'deadline.json'
    $temporary = $path + '.' + [guid]::NewGuid().ToString('N') + '.tmp'
    [IO.File]::WriteAllText($temporary, ($record | ConvertTo-Json), [Text.Encoding]::UTF8)
    try {
        if ([IO.File]::Exists($path)) { [IO.File]::Replace($temporary, $path, [NullString]::Value) }
        else { [IO.File]::Move($temporary, $path) }
    } finally { if ([IO.File]::Exists($temporary)) { [IO.File]::Delete($temporary) } }
    return $StartedTicks
}
function Read-QualificationDeadline {
    param([string]$Directory, [string]$RunId)
    $path = Join-Path $Directory 'deadline.json'
    if (-not [IO.File]::Exists($path)) { return $null }
    # Replace can briefly deny even a delete-sharing read on Windows. Retry only
    # sharing/lock violations, for at most 200ms; never turn failure into "done".
    # Deadline evaluation below still uses the original monotonic start time.
    $exchangeWait = [Diagnostics.Stopwatch]::StartNew()
    while ($true) {
        $reader = $null
        $stream = $null
        try {
            $stream = [IO.File]::Open($path, [IO.FileMode]::Open, [IO.FileAccess]::Read, ([IO.FileShare]::ReadWrite -bor [IO.FileShare]::Delete))
            $reader = [IO.StreamReader]::new($stream)
            $record = $reader.ReadToEnd() | ConvertFrom-Json
            break
        } catch {
            $base = $_.Exception.GetBaseException()
            $code = $base.HResult -band 0xffff
            if ($base -isnot [IO.IOException] -or $code -notin @(32,33) -or $exchangeWait.ElapsedMilliseconds -ge 200) {
                throw ('Invalid qualification stage record: ' + $_.Exception.Message)
            }
            Start-Sleep -Milliseconds 5
        } finally {
            if ($reader) { $reader.Dispose() }
            elseif ($stream) { $stream.Dispose() }
        }
    }
    if ($record.run_id -ne $RunId) { throw 'Qualification stage belongs to another run' }
    if (-not $record.active) { return $null }
    if ($record.frequency -ne [Diagnostics.Stopwatch]::Frequency -or $record.limit_ms -le 0 -or $record.limit_ms -gt 8000 -or $record.started_ticks -le 0) {
        throw 'Invalid qualification deadline bounds'
    }
    $elapsed = ([Diagnostics.Stopwatch]::GetTimestamp() - $record.started_ticks) * 1000.0 / $record.frequency
    if ($elapsed -lt 0) { throw 'Qualification stage timestamp is in the future' }
    if ($elapsed -ge $record.limit_ms) {
        return @{stage=$record.stage; elapsed_ms=$elapsed; limit_ms=$record.limit_ms;
            runner_pid=$record.runner_pid; runner_start_ticks=$record.runner_start_ticks}
    }
    return $null
}

# A process can exit between Get-Process and reading StartTime. Exited is not
# owned/live; unavailable identity on a still-live process remains an error.
function Test-QualificationProcessIdentity {
    param($Process, [long]$StartedTicks)
    if ($null -eq $Process) { return $false }
    try {
        if ($Process.HasExited) { return $false }
        $started = $Process.StartTime
        if ($null -eq $started) {
            if ($Process.HasExited) { return $false }
            throw 'Live process start time unavailable'
        }
        return $started.ToUniversalTime().Ticks -eq $StartedTicks
    } catch {
        $failure = $_
        if ($Process.HasExited) { return $false }
        throw $failure
    }
}

# UIA event registration/removal belongs on one windowless MTA thread.
function Assert-QualificationAutomationThread {
    if ([Threading.Thread]::CurrentThread.GetApartmentState() -ne [Threading.ApartmentState]::MTA) {
        throw 'UI Automation requires an explicit MTA interactive runner'
    }
    return [Threading.Thread]::CurrentThread.ManagedThreadId
}
