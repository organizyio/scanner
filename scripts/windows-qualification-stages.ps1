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
    try {
        $stream = [IO.File]::Open($path, [IO.FileMode]::Open, [IO.FileAccess]::Read, ([IO.FileShare]::ReadWrite -bor [IO.FileShare]::Delete))
        $reader = [IO.StreamReader]::new($stream)
        try { $record = $reader.ReadToEnd() | ConvertFrom-Json }
        finally { $reader.Dispose() }
    }
    catch { throw ('Invalid qualification stage record: ' + $_.Exception.Message) }
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
