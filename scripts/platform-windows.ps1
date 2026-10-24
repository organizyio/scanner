# Native interactive evidence runner. Use only synthetic fixture/binary paths.
param(
    [Parameter(Mandatory=$true)][string]$Binary,
    [Parameter(Mandatory=$true)][string]$Fixture,
    [string]$CliBinary,
    [ValidatePattern('^(start|pause|resume|cancel|restart|settings|picker|announcements|textselection|keyboard|lifecycle|profile|processrestart|quit|quitshortcut)(,(start|pause|resume|cancel|restart|settings|picker|announcements|textselection|keyboard|lifecycle|profile|processrestart|quit|quitshortcut))*$')]
    [string]$Actions='start',
    [switch]$Interactive,
    [string]$RunId=''
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
    $RunId = [guid]::NewGuid().ToString('N')
    $task = 'ScannerQualification-' + $RunId
    $report = Join-Path $Fixture 'windows-ui/report.json'
    if (Test-Path -LiteralPath $report) {
        $saved = Join-Path $Fixture ('windows-ui/report-previous-' + [guid]::NewGuid().ToString('N') + '.json')
        Move-Item -LiteralPath $report -Destination $saved
    }
    $argsText = '-NoProfile -NonInteractive -WindowStyle Hidden -File "' + $PSCommandPath + '" -Binary "' + $Binary + '" -Fixture "' + $Fixture + '" -Interactive'
    if ($CliBinary) { $argsText += ' -CliBinary "' + (Resolve-Path -LiteralPath $CliBinary).Path + '"' }
    $argsText += ' -Actions "' + $Actions + '" -RunId "' + $RunId + '"'
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
        $pidFile = Join-Path $Fixture 'windows-ui/process.json'
        if (Test-Path -LiteralPath $pidFile) {
            $owned = Get-Content -LiteralPath $pidFile -Raw | ConvertFrom-Json
            if ($owned.run_id -eq $RunId) {
                $left = Get-Process -Id $owned.pid -ErrorAction SilentlyContinue
                if ($left -and $left.Path -eq $Binary -and $left.StartTime.ToUniversalTime().Ticks -eq $owned.start_ticks) {
                    Stop-Process -Id $left.Id
                }
            }
        }
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
    public delegate bool WindowVisitor(IntPtr h, IntPtr state);
    [DllImport("user32.dll")] public static extern bool EnumWindows(WindowVisitor visitor, IntPtr state);
    [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h, System.Text.StringBuilder text, int size);
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
    public static IntPtr Find(uint owner, string title) {
        IntPtr found=IntPtr.Zero;
        EnumWindows((h,state)=> { uint pid; GetWindowThreadProcessId(h,out pid);
            var text=new System.Text.StringBuilder(512); GetWindowText(h,text,text.Capacity);
            if(pid==owner && IsWindowVisible(h) && text.ToString()==title) { found=h; return false; }
            return true;
        },IntPtr.Zero);
        return found;
    }
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out Rect r);
    [StructLayout(LayoutKind.Sequential)] public struct Point { public int X,Y; }
    [DllImport("kernel32.dll", SetLastError=true)] public static extern bool GetExitCodeProcess(IntPtr handle, out uint code);
    [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr h, out Rect r);
    [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr h, ref Point p);
    [DllImport("user32.dll")] public static extern uint GetDpiForWindow(IntPtr h);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint x, uint y, uint data, UIntPtr extra);
}
'@
    $process = Start-Process -FilePath $Binary -PassThru -RedirectStandardError (Join-Path $out 'stderr.txt')
    @{run_id=$RunId; pid=$process.Id; start_ticks=$process.StartTime.ToUniversalTime().Ticks} | ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $out 'process.json')
    $deadline = (Get-Date).AddSeconds(30)
    do {
        Start-Sleep -Milliseconds 250
        $process.Refresh()
        $handle = [QualificationWindow]::Find($process.Id, 'Scanner')
    } while ($handle -eq 0 -and -not $process.HasExited -and (Get-Date) -lt $deadline)
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
        $origin = New-Object QualificationWindow+Point
        if (-not [QualificationWindow]::GetClientRect($current, [ref]$bounds) -or -not [QualificationWindow]::ClientToScreen($current, [ref]$origin)) { throw 'Owned client capture bounds unavailable' }
        if ($bounds.Right -lt 300 -or $bounds.Bottom -lt 200) { throw 'Owned client window is too small for qualification' }
        $image = [Drawing.Bitmap]::new($bounds.Right, $bounds.Bottom)
        $canvas = [Drawing.Graphics]::FromImage($image)
        try {
            $canvas.CopyFromScreen($origin.X,$origin.Y,0,0,$image.Size)
            if ([QualificationWindow]::GetForegroundWindow() -ne $current) { throw 'Focus changed during capture; image discarded' }
            $image.Save((Join-Path $out $name), [Drawing.Imaging.ImageFormat]::Png)
        } finally { $canvas.Dispose(); $image.Dispose() }
    }
    function Require-OwnedForeground {
        [uint32]$owner = 0
        [QualificationWindow]::GetWindowThreadProcessId([QualificationWindow]::GetForegroundWindow(), [ref]$owner) | Out-Null
        if ($owner -ne $process.Id) { throw ('Focus changed; refusing native action at ' + $script:nativeStage + '; owned=' + $process.Id + '; foreground=' + $owner) }
    }
    function Find-NativeControl([string]$name) {
        $script:nativeStage = $name
        Require-OwnedForeground
        $condition = [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::NameProperty, $name)
        $deadline = (Get-Date).AddSeconds(5)
        do {
            Require-OwnedForeground
            $control = $element.FindFirst([System.Windows.Automation.TreeScope]::Subtree, $condition)
            if ($null -ne $control) { return $control }
            Start-Sleep -Milliseconds 100
        } while ((Get-Date) -lt $deadline)
        throw ('Native accessible control unavailable: ' + $name)
    }
    function Invoke-NativeControl([string]$name) {
        $control = Find-NativeControl $name
        $invoke = $control.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
        $invoke.Invoke()
    }
    function Native-Value($control) {
        $pattern = $null
        if ($control.TryGetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern, [ref]$pattern)) { return $pattern.Current.Value }
        if ($control.TryGetCurrentPattern([System.Windows.Automation.TextPattern]::Pattern, [ref]$pattern)) { return $pattern.DocumentRange.GetText(-1) }
        if ($control.TryGetCurrentPattern([System.Windows.Automation.LegacyIAccessiblePattern]::Pattern, [ref]$pattern)) { return $pattern.Current.Value }
        return $control.Current.Name
    }
    function Save-NativeSnapshot([string]$name) {
        $nodes = $element.FindAll([System.Windows.Automation.TreeScope]::Subtree, [System.Windows.Automation.Condition]::TrueCondition)
        $values = @($nodes | ForEach-Object {
            $selection = $null
            $selected = $null
            if ($_.TryGetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern, [ref]$selection)) { $selected = $selection.Current.IsSelected }
            @{name=$_.Current.Name; role=$_.Current.ControlType.ProgrammaticName; value=(Native-Value $_); focused=$_.Current.HasKeyboardFocus; selected=$selected}
        })
        $values | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 (Join-Path $out $name)
        return $values
    }
    function Read-NativeStatus {
        Require-OwnedForeground
        $nodes = $element.FindAll([System.Windows.Automation.TreeScope]::Subtree, [System.Windows.Automation.Condition]::TrueCondition)
        foreach ($node in $nodes) {
            $text = Native-Value $node
            if ($text -match '^(idle|running|paused|canceling|canceled|finished|error) \| files=') { return $text }
        }
        return ''
    }
    function Wait-NativeStatus([string]$prefix) {
        $deadline = (Get-Date).AddSeconds(10)
        do {
            $status = Read-NativeStatus
            if ($status -like ($prefix + '*')) { return $status }
            Start-Sleep -Milliseconds 100
        } while ((Get-Date) -lt $deadline)
        throw ('Expected ' + $prefix + ', observed native status: ' + $status)
    }
    $qualified = @{}
    $quitObserved = $false
    # Use native UIA Invoke actions: changed layouts cannot click another control.
    $names = @{start='Start'; restart='Start (Resume)'; pause='Pause'; resume='Resume'; cancel='Cancel'; settings='Settings'}
    $sequence = 0
    foreach ($action in $Actions.Split(',')) {
        if ($action -eq 'quit' -or $action -eq 'quitshortcut') {
            if ($action -ne $Actions.Split(',')[-1]) { throw 'Quit must be the final action' }
            Require-OwnedForeground
            $exitHandle = $process.Handle
            $quitTimer = [Diagnostics.Stopwatch]::StartNew()
            @{pid=$process.Id; started_utc=[DateTime]::UtcNow.ToString('o')} | ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $out 'quit-start.json')
            $quitInvokeError = $null
            try {
                if ($action -eq 'quit') { Invoke-NativeControl 'Quit' }
                else { [System.Windows.Forms.SendKeys]::SendWait('^q') }
            } catch {
                # UIA can disconnect while an invoked Quit is returning. Only a
                # verified exit-zero process below can establish successful exit.
                $quitInvokeError = $_.Exception.Message
            }
            if (-not $process.WaitForExit(8000)) { throw 'Owned native Quit did not terminate within eight seconds' }
            [uint32]$nativeExitCode = 259
            if (-not [QualificationWindow]::GetExitCodeProcess($exitHandle, [ref]$nativeExitCode) -or $nativeExitCode -ne 0) { throw ('Owned native Quit returned ' + $nativeExitCode) }
            if ($quitInvokeError) { throw ('Exit zero observed, but native Quit Invoke failed: ' + $quitInvokeError) }
            $quitObserved = $true
            $qualified.quit = @{native_action=$action; exit_code=$nativeExitCode; forced=$false; elapsed_ms=$quitTimer.ElapsedMilliseconds; invoke_error=$quitInvokeError}
        } elseif ($action -eq 'announcements') {
            $references = @([System.Windows.Automation.AutomationElement].Assembly.Location, [System.Windows.Automation.AutomationEvent].Assembly.Location)
            Add-Type -ReferencedAssemblies $references @'
using System;
using System.Collections.Concurrent;
using System.Windows.Automation;
public static class ScannerLiveEvents {
    static AutomationElement root;
    static AutomationEventHandler handler;
    static AutomationEvent live = AutomationEvent.LookupById(20024);
    public static ConcurrentQueue<string> Messages = new ConcurrentQueue<string>();
    public static void Start(AutomationElement element, int pid) {
        root = element;
        handler = (sender,args) => {
            try {
                var node = sender as AutomationElement;
                if (node != null && node.Current.ProcessId == pid) Messages.Enqueue(node.Current.Name);
            } catch (ElementNotAvailableException) { }
        };
        Automation.AddAutomationEventHandler(live, root, TreeScope.Subtree, handler);
    }
    public static void Stop() {
        if (handler != null) {
            try { Automation.RemoveAutomationEventHandler(live, root, handler); }
            catch (ElementNotAvailableException) { }
            finally { handler = null; root = null; }
        }
    }
}
'@
            [ScannerLiveEvents]::Start($element, $process.Id)
            Invoke-NativeControl 'Start'
            Wait-NativeStatus 'finished' | Out-Null
            $deadline = (Get-Date).AddSeconds(4)
            do {
                $messages = [ScannerLiveEvents]::Messages.ToArray()
                if (@($messages | Where-Object { $_ -like 'finished | files=*' }).Count) { break }
                Start-Sleep -Milliseconds 100
            } while ((Get-Date) -lt $deadline)
            if (-not @($messages | Where-Object { $_ -like 'finished | files=*' }).Count) { throw 'Native finished live-region event missing' }
            Invoke-NativeControl 'Add Folder'
            Start-Sleep -Milliseconds 500
            $entry = Find-NativeControl 'Folder path: type a path, then Enter or Go'
            $value = $entry.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern)
            $invalid = Join-Path $Fixture 'nonexistent-folder'
            $value.SetValue($invalid)
            Invoke-NativeControl 'Go'
            $deadline = (Get-Date).AddSeconds(4)
            do {
                $messages = [ScannerLiveEvents]::Messages.ToArray()
                if (@($messages | Where-Object { $_ -like 'Cannot open folder:*' -and $_.Contains($invalid) }).Count) { break }
                Start-Sleep -Milliseconds 100
            } while ((Get-Date) -lt $deadline)
            $messages | ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $out 'live-region-events.json')
            if (-not @($messages | Where-Object { $_ -like 'Cannot open folder:*' -and $_.Contains($invalid) }).Count) { throw 'Native affected-folder live-region event missing' }
            # Keep the native observer attached through app Quit, like an active reader.
            $qualified.announcements = @{finished_event=$true; affected_folder_error_event=$true; spoken_reader=$false}
            Invoke-NativeControl 'Cancel'
        } elseif ($action -eq 'textselection') {
            Invoke-NativeControl 'Add Folder'
            Start-Sleep -Milliseconds 500
            $entry = Find-NativeControl 'Folder path: type a path, then Enter or Go'
            $entry.SetFocus()
            $value = $entry.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern)
            $unicode = 'a' + [char]0x00E9 + [char]::ConvertFromUtf32(0x1F642) + 'z'
            $expected = [string][char]0x00E9 + [char]::ConvertFromUtf32(0x1F642)
            $value.SetValue($unicode)
            Start-Sleep -Milliseconds 500
            $text = $entry.GetCurrentPattern([System.Windows.Automation.TextPattern]::Pattern)
            $range = $text.DocumentRange.Clone()
            $range.MoveEndpointByUnit([System.Windows.Automation.Text.TextPatternRangeEndpoint]::Start, [System.Windows.Automation.Text.TextUnit]::Character, 1) | Out-Null
            $range.MoveEndpointByUnit([System.Windows.Automation.Text.TextPatternRangeEndpoint]::End, [System.Windows.Automation.Text.TextUnit]::Character, -1) | Out-Null
            $range.Select()
            $deadline = (Get-Date).AddSeconds(3)
            do {
                $selection = $text.GetSelection()
                if ($selection.Count -eq 1 -and $selection[0].GetText(-1) -eq $expected) { break }
                Start-Sleep -Milliseconds 100
            } while ((Get-Date) -lt $deadline)
            if ($selection.Count -ne 1 -or $selection[0].GetText(-1) -ne $expected) { throw 'Native Unicode selection did not round trip' }
            $caret = $text.DocumentRange.Clone()
            $caret.MoveEndpointByUnit([System.Windows.Automation.Text.TextPatternRangeEndpoint]::Start, [System.Windows.Automation.Text.TextUnit]::Character, 2) | Out-Null
            $caret.MoveEndpointByRange([System.Windows.Automation.Text.TextPatternRangeEndpoint]::End, $caret, [System.Windows.Automation.Text.TextPatternRangeEndpoint]::Start)
            $caret.Select()
            Start-Sleep -Milliseconds 500
            $observed = $text.GetSelection()
            if ($observed.Count -ne 1 -or $observed[0].GetText(-1) -ne '' -or $observed[0].CompareEndpoints([System.Windows.Automation.Text.TextPatternRangeEndpoint]::Start, $caret, [System.Windows.Automation.Text.TextPatternRangeEndpoint]::Start) -ne 0) { throw 'Native caret did not round trip' }
            if ($value.Current.Value -ne $unicode) { throw 'Selection changed field contents' }
            Save-WindowCapture 'text-selection.png'
            $qualified.textselection = @{unicode_selection=$true; caret=$true; value_unchanged=$true; spoken_reader=$false}
            Invoke-NativeControl 'Cancel'
        } elseif ($action -eq 'picker') {
            Invoke-NativeControl 'Add Folder'
            Start-Sleep -Milliseconds 500
            $entry = Find-NativeControl 'Folder path: type a path, then Enter or Go'
            $entry.SetFocus()
            Start-Sleep -Milliseconds 300
            if (-not $entry.Current.HasKeyboardFocus) { throw 'Native folder path focus did not reach the control' }
            $value = $entry.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern)
            $invalid = Join-Path $Fixture 'nonexistent-folder'
            $value.SetValue($invalid)
            Invoke-NativeControl 'Go'
            Start-Sleep -Milliseconds 500
            $errors = Save-NativeSnapshot 'picker-error.json'
            if (-not (@($errors | Where-Object { $_.value -like '*Cannot open folder*' -and $_.value.Contains($invalid) }).Count)) { throw 'Native error status missing affected folder' }
            Save-WindowCapture 'picker-error.png'
            $selected = Get-ChildItem -LiteralPath (Join-Path $Fixture 'scan-root') -Directory | Select-Object -First 1
            $value.SetValue($selected.FullName)
            Invoke-NativeControl 'Go'
            Start-Sleep -Milliseconds 500
            Invoke-NativeControl 'Select Folder'
            Start-Sleep -Milliseconds 500
            $settings = Join-Path $Fixture 'config/ScannerApp/settings.yaml'
            if (-not ([IO.File]::ReadAllText($settings).Contains($selected.FullName))) { throw 'Native selected root was not saved' }
            $qualified.picker = @{native_focus=$true; native_value_edit=$true; error_status=$true; selected_root_persisted=$true}
        } elseif ($action -eq 'processrestart') {
            $settings = Join-Path $Fixture 'config/ScannerApp/settings.yaml'
            $settingsBefore = (Get-FileHash -LiteralPath $settings -Algorithm SHA256).Hash
            if (-not (Test-Path -LiteralPath (Join-Path $Fixture 'checkpoint.json'))) { throw 'No synthetic checkpoint available for restart' }
            Stop-Process -Id $process.Id
            if (-not $process.WaitForExit(5000)) { throw 'Owned previous GUI did not stop before restart' }
            $process = Start-Process -FilePath $Binary -PassThru -RedirectStandardError (Join-Path $out 'restart.stderr.txt')
            @{run_id=$RunId; pid=$process.Id; start_ticks=$process.StartTime.ToUniversalTime().Ticks} | ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $out 'process.json')
            $deadline = (Get-Date).AddSeconds(30)
            do {
                Start-Sleep -Milliseconds 100
                $process.Refresh()
                $handle = [QualificationWindow]::Find($process.Id, 'Scanner')
            } while ($handle -eq 0 -and -not $process.HasExited -and (Get-Date) -lt $deadline)
            if ($handle -eq 0) { throw 'Restart produced no native Scanner window' }
            [QualificationWindow]::SetForegroundWindow($handle) | Out-Null
            Start-Sleep -Milliseconds 500
            Require-OwnedForeground
            $element = [System.Windows.Automation.AutomationElement]::FromHandle($handle)
            Invoke-NativeControl 'Start (Resume)'
            $status = Wait-NativeStatus 'finished'
            if ($status -notlike '*Resumed partial inventory*') { throw 'Restart did not expose partial checkpoint semantics' }
            if ((Get-FileHash -LiteralPath $settings -Algorithm SHA256).Hash -ne $settingsBefore) { throw 'Restart altered synthetic settings' }
            $qualified.processrestart = @{settings_preserved=$true; partial_resume_status=$status}
        } elseif ($action -eq 'profile') {
            Invoke-NativeControl 'Profile: Qualification'
            Start-Sleep -Milliseconds 500
            Invoke-NativeControl 'Use profile Secondary'
            Start-Sleep -Milliseconds 500
            $settings = Join-Path $Fixture 'config/ScannerApp/settings.yaml'
            if ([IO.File]::ReadAllText($settings) -notmatch 'active_profile: Secondary') { throw 'Profile choice was not saved' }
            Invoke-NativeControl 'Profile: Secondary'
            Start-Sleep -Milliseconds 500
            Invoke-NativeControl 'Use profile Qualification'
            Start-Sleep -Milliseconds 500
            if ([IO.File]::ReadAllText($settings) -notmatch 'active_profile: Qualification') { throw 'Original synthetic profile was not restored' }
            $qualified.profile = 'native choice, persistence and restore passed'
        } elseif ($action -eq 'keyboard') {
            Require-OwnedForeground
            [System.Windows.Forms.SendKeys]::SendWait('^l')
            Require-OwnedForeground
            [System.Windows.Forms.SendKeys]::SendWait('{DOWN} ')
            Start-Sleep -Milliseconds 500
            $snapshot = Save-NativeSnapshot 'keyboard.json'
            $selectedRows = @($snapshot | Where-Object { $_.role -eq 'ControlType.TreeItem' -and $_.selected -eq $true })
            $focused = @($snapshot | Where-Object { $_.focused -and ($_.role -eq 'ControlType.Tree' -or $_.role -eq 'ControlType.TreeItem') })
            if ($selectedRows.Count -ne 1 -or $focused.Count -lt 1) { throw 'Native keyboard tree selection/focus not exposed' }
            $selectedPath = $selectedRows[0].name -replace ' \((enabled|disabled)\)$',''
            $details = @($snapshot | Where-Object { $_.value -and $_.value.Contains($selectedPath) -and ($_.value.Contains('Files:') -or $_.value.Contains('(no data yet)')) })
            if ($details.Count -lt 1) { throw 'Keyboard selection did not update accessible folder details' }
            $qualified.keyboard = @{focused=$focused[0].name; selected=$selectedRows[0].name; details_match=$true; visible_focus='review own-window screenshot'}
        } elseif ($action -eq 'lifecycle') {
            Require-OwnedForeground
            [System.Windows.Forms.SendKeys]::SendWait('^s^p')
            $paused = Wait-NativeStatus 'paused'
            Save-WindowCapture 'paused.png'
            Require-OwnedForeground
            [System.Windows.Forms.SendKeys]::SendWait('^r^k')
            $canceled = Wait-NativeStatus 'canceled'
            $qualified.lifecycle = @{paused=$paused; canceled=$canceled}
        } else {
            Invoke-NativeControl $names[$action]
        }
        Start-Sleep -Seconds 2
        if (-not $quitObserved) { Save-WindowCapture ('action-{0}-{1}.png' -f $sequence,$action) }
        if ($action -eq 'settings') { $handle = [QualificationWindow]::GetForegroundWindow() }
        $sequence++
    }
    # Input belongs to explicit cases above; final capture never activates a
    # newly focused control or changes settings after its asserted result.
    if (-not $quitObserved) {
        Require-OwnedForeground
        Save-WindowCapture 'window.png'
    }
    if ((Get-Content -LiteralPath (Join-Path $out 'stderr.txt') -Raw) -match 'Error in Fyne call thread') { throw 'Fyne wrong-thread diagnostic observed' }
    @{system='Windows'; architecture=$env:PROCESSOR_ARCHITECTURE; interactive=$true;
      input=$Actions; qualified=$qualified; accessible_controls=$controls; gui_qualification='pending-review'; release_acceptance=$false} |
        ConvertTo-Json -Depth 6 | Set-Content -Encoding UTF8 (Join-Path $out 'report.json')
} catch {
    @{gui_qualification='pending'; error=$_.Exception.Message; release_acceptance=$false} |
        ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $out 'report.json')
    throw
} finally {
    try { if ('ScannerLiveEvents' -as [type]) { [ScannerLiveEvents]::Stop() } }
    finally { if ($process -and -not $process.HasExited) { Stop-Process -Id $process.Id } }
}
