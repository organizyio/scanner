# Native interactive evidence runner. Use only synthetic fixture/binary paths.
param(
    [Parameter(Mandatory=$true)][string]$Binary,
    [Parameter(Mandatory=$true)][string]$Fixture,
    [string]$CliBinary,
    [ValidatePattern('^(start|pause|resume|cancel|restart|settings)(,(start|pause|resume|cancel|restart|settings))*$')]
    [string]$Actions='start',
    [switch]$Interactive
)
$ErrorActionPreference = 'Stop'
$Binary = (Resolve-Path -LiteralPath $Binary).Path
$Fixture = (Resolve-Path -LiteralPath $Fixture).Path
if (-not (Test-Path -LiteralPath (Join-Path $Fixture 'qualification-fixture.json'))) {
    throw 'Prepare an isolated synthetic fixture before GUI launch'
}
if (-not $Interactive) {
    # SSH runs outside the console. A temporary same-user interactive token task
    # uses the existing signed-in session without passwords or policy changes.
    $task = 'ScannerQualification-' + [guid]::NewGuid().ToString('N')
    $report = Join-Path $Fixture 'windows-ui/report.json'
    if (Test-Path -LiteralPath $report) {
        $saved = Join-Path $Fixture ('windows-ui/report-previous-' + [guid]::NewGuid().ToString('N') + '.json')
        Move-Item -LiteralPath $report -Destination $saved
    }
    $argsText = '-NoProfile -NonInteractive -File "' + $PSCommandPath + '" -Binary "' + $Binary + '" -Fixture "' + $Fixture + '" -Interactive'
    if ($CliBinary) { $argsText += ' -CliBinary "' + (Resolve-Path -LiteralPath $CliBinary).Path + '"' }
    $argsText += ' -Actions "' + $Actions + '"'
    $action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument $argsText
    $principal = New-ScheduledTaskPrincipal -UserId ([Security.Principal.WindowsIdentity]::GetCurrent().Name) -LogonType Interactive
    try {
        Register-ScheduledTask -TaskName $task -Action $action -Principal $principal | Out-Null
        Start-ScheduledTask -TaskName $task
        $deadline = (Get-Date).AddSeconds(90)
        do {
            Start-Sleep -Milliseconds 500
            $state = (Get-ScheduledTask -TaskName $task).State
        } while ((-not (Test-Path -LiteralPath $report) -or $state -eq 'Running') -and (Get-Date) -lt $deadline)
        if ($state -eq 'Running') { throw 'Interactive qualification timed out' }
        $exitCode = (Get-ScheduledTaskInfo -TaskName $task).LastTaskResult
        if ($exitCode -ne 0) { throw ('Interactive runner failed with task exit code ' + $exitCode) }
        if (-not (Test-Path -LiteralPath (Join-Path $Fixture 'windows-ui/report.json'))) {
            throw 'Interactive desktop unavailable or runner failed; qualification pending'
        }
        Get-Content -LiteralPath (Join-Path $Fixture 'windows-ui/report.json')
    } finally {
        Stop-ScheduledTask -TaskName $task -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $task -Confirm:$false -ErrorAction SilentlyContinue
    }
    exit
}

$out = Join-Path $Fixture 'windows-ui'
New-Item -ItemType Directory -Path $out -Force | Out-Null
$process = $null
try {
    if (-not [Environment]::UserInteractive) { throw 'No interactive desktop token' }
    $python = (Get-Command python).Source
    foreach ($item in @{'HOME'='home'; 'USERPROFILE'='home'; 'APPDATA'='config'; 'LOCALAPPDATA'='local'; 'XDG_CONFIG_HOME'='config'; 'XDG_CACHE_HOME'='cache'; 'XDG_DATA_HOME'='data'}.GetEnumerator()) {
        $directory = Join-Path $Fixture $item.Value
        New-Item -ItemType Directory -Path $directory -Force | Out-Null
        [Environment]::SetEnvironmentVariable($item.Key, $directory, 'Process')
    }
    if ($CliBinary) {
        & $python (Join-Path (Split-Path $PSCommandPath) 'platform-qualification.py') cli --fixture $Fixture --binary $CliBinary 2>&1 |
            Out-File -Encoding UTF8 (Join-Path $out 'console-cli.txt')
        # A pending CLI gate is evidence, not permission to claim qualification.
    }
    Add-Type -AssemblyName UIAutomationClient
    Add-Type -AssemblyName UIAutomationTypes
    Add-Type -AssemblyName System.Windows.Forms
    Add-Type -AssemblyName System.Drawing
    Add-Type @'
using System;
using System.Runtime.InteropServices;
public class QualificationWindow {
    [StructLayout(LayoutKind.Sequential)] public struct Rect { public int Left,Top,Right,Bottom; }
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out Rect r);
    [DllImport("user32.dll")] public static extern uint GetDpiForWindow(IntPtr h);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint x, uint y, uint data, UIntPtr extra);
}
'@
    $process = Start-Process -FilePath $Binary -PassThru -RedirectStandardError (Join-Path $out 'stderr.txt')
    $deadline = (Get-Date).AddSeconds(30)
    do {
        Start-Sleep -Milliseconds 250
        $process.Refresh()
    } while ($process.MainWindowHandle -eq 0 -and -not $process.HasExited -and (Get-Date) -lt $deadline)
    $handle = $process.MainWindowHandle
    if ($handle -eq 0) { throw 'Fyne produced no interactive window' }
    [QualificationWindow]::SetForegroundWindow($handle) | Out-Null
    Start-Sleep -Milliseconds 500
    if ([QualificationWindow]::GetForegroundWindow() -ne $handle) { throw 'Cannot focus synthetic window; no input sent' }
    $element = [System.Windows.Automation.AutomationElement]::FromHandle($handle)
    $all = $element.FindAll([System.Windows.Automation.TreeScope]::Subtree, [System.Windows.Automation.Condition]::TrueCondition)
    $controls = @($all | ForEach-Object { @{name=$_.Current.Name; role=$_.Current.ControlType.ProgrammaticName; keyboard_focusable=$_.Current.IsKeyboardFocusable} })
    $controls | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 (Join-Path $out 'accessibility.json')
    function Save-WindowCapture([string]$name) {
        $current = [QualificationWindow]::GetForegroundWindow()
        [uint32]$owner = 0
        [QualificationWindow]::GetWindowThreadProcessId($current, [ref]$owner) | Out-Null
        if ($owner -ne $process.Id) { throw 'Focus left synthetic process; refusing screenshot' }
        $bounds = New-Object QualificationWindow+Rect
        [QualificationWindow]::GetWindowRect($current, [ref]$bounds) | Out-Null
        $image = [Drawing.Bitmap]::new(($bounds.Right-$bounds.Left), ($bounds.Bottom-$bounds.Top))
        $canvas = [Drawing.Graphics]::FromImage($image)
        try {
            $canvas.CopyFromScreen($bounds.Left,$bounds.Top,0,0,$image.Size)
            $image.Save((Join-Path $out $name), [Drawing.Imaging.ImageFormat]::Png)
        } finally { $canvas.Dispose(); $image.Dispose() }
    }
    # Coordinates refer to visible controls in the isolated foreground window.
    $positions = @{start=30; restart=120; pause=210; resume=280; cancel=350; settings=625}
    $sequence = 0
    foreach ($action in $Actions.Split(',')) {
        if ([QualificationWindow]::GetForegroundWindow() -ne $handle) { throw 'Focus changed; no click sent' }
        $startRect = New-Object QualificationWindow+Rect
        [QualificationWindow]::GetWindowRect($handle, [ref]$startRect) | Out-Null
        $scale = [QualificationWindow]::GetDpiForWindow($handle) / 96.0
        [QualificationWindow]::SetCursorPos(($startRect.Left + [int]($positions[$action]*$scale)), ($startRect.Top + [int](50*$scale))) | Out-Null
        [QualificationWindow]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
        Start-Sleep -Milliseconds 80
        [QualificationWindow]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
        Start-Sleep -Seconds 2
        Save-WindowCapture ('action-{0}-{1}.png' -f $sequence,$action)
        if ($action -eq 'settings') { $handle = [QualificationWindow]::GetForegroundWindow() }
        $sequence++
    }
    # Genuine keyboard events delivered to the focused app, not widget test calls.
    [System.Windows.Forms.SendKeys]::SendWait('{TAB}')
    Start-Sleep -Milliseconds 250
    [System.Windows.Forms.SendKeys]::SendWait(' ')
    Start-Sleep -Seconds 2
    if ([QualificationWindow]::GetForegroundWindow() -ne $handle) { throw 'Focus changed; refusing screenshot of unrelated content' }
    $rect = New-Object QualificationWindow+Rect
    [QualificationWindow]::GetWindowRect($handle, [ref]$rect) | Out-Null
    $bitmap = [Drawing.Bitmap]::new(($rect.Right-$rect.Left), ($rect.Bottom-$rect.Top))
    $graphics = [Drawing.Graphics]::FromImage($bitmap)
    try {
        $graphics.CopyFromScreen($rect.Left, $rect.Top, 0, 0, $bitmap.Size)
        $bitmap.Save((Join-Path $out 'window.png'), [Drawing.Imaging.ImageFormat]::Png)
    } finally { $graphics.Dispose(); $bitmap.Dispose() }
    @{system='Windows'; architecture=$env:PROCESSOR_ARCHITECTURE; interactive=$true;
      input=($Actions+',Tab,Space'); accessible_controls=$controls; gui_qualification='pending-review'; release_acceptance=$false} |
        ConvertTo-Json -Depth 6 | Set-Content -Encoding UTF8 (Join-Path $out 'report.json')
} catch {
    @{gui_qualification='pending'; error=$_.Exception.Message; release_acceptance=$false} |
        ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $out 'report.json')
    throw
} finally {
    if ($process -and -not $process.HasExited) { Stop-Process -Id $process.Id }
}
