# Shows a claude-notify toast on Windows and, for one with buttons, waits for the answer.
#
# Input: the CLAUDE_NOTIFY_REQUEST environment variable, a JSON object with
#   title, body
#   sound     "default", "none", or the path of an mp3 to play instead of the Windows sound
#   icon      png shown as the app icon
#   actions   "" (plain), "permission" (Allow / Deny) or "reply" (Reply field)
#   wait      seconds to wait for an answer before withdrawing the toast
#   id        tag of the toast
#   response  file the answer goes to: {"action": "allow" | "deny" | "reply" | "open", "text": ...}
#   parent    pid of the hook; when it goes away the toast is withdrawn
#   cache     folder for the compiled ToastWaiter.cs (needed with actions)
#
# Runs under Windows PowerShell 5.1: PowerShell 7 can't load the WinRT notification types.
# Keep this file ASCII, 5.1 reads BOM-less scripts in the ANSI code page.

$ErrorActionPreference = 'Stop'
$req = $env:CLAUDE_NOTIFY_REQUEST | ConvertFrom-Json

# Toasts take the name and icon registered for their app id
$AppId = 'ClaudeNotify.Hook'
$key = "HKCU:\Software\Classes\AppUserModelId\$AppId"
if (-not (Test-Path $key)) { New-Item -Path $key -Force | Out-Null }
Set-ItemProperty -Path $key -Name DisplayName -Value 'Claude Notify'
if ($req.icon) { Set-ItemProperty -Path $key -Name IconUri -Value $req.icon }

$null = [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime]
$null = [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime]

function Esc([string]$s) { [Security.SecurityElement]::Escape($s) }

$actions = switch ($req.actions) {
    'permission' {
        '<actions>' +
        '<action content="Allow" arguments="allow" activationType="foreground"/>' +
        '<action content="Deny" arguments="deny" activationType="foreground"/>' +
        '</actions>'
    }
    'reply' {
        '<actions>' +
        '<input id="reply" type="text" placeHolderContent="Reply"/>' +
        '<action content="Send" arguments="reply" activationType="foreground" hint-inputId="reply"/>' +
        '</actions>'
    }
    default { '' }
}
$silent = if ($req.sound -eq 'default') { 'false' } else { 'true' }
$duration = if ($req.actions) { 'long' } else { 'short' }

$doc = New-Object Windows.Data.Xml.Dom.XmlDocument
$doc.LoadXml(@"
<toast launch="open" activationType="foreground" duration="$duration">
  <visual><binding template="ToastGeneric"><text>$(Esc $req.title)</text><text>$(Esc $req.body)</text></binding></visual>
  <audio silent="$silent"/>
  $actions
</toast>
"@)
$toast = New-Object Windows.UI.Notifications.ToastNotification $doc
$toast.Tag = "$($req.id)"
$toast.Group = 'claude-notify'
$notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($AppId)

# ToastWaiter.cs compiled into the cache, once per version of the source
function Get-ToastWaiter {
    $src = Join-Path $PSScriptRoot 'ToastWaiter.cs'
    $hash = (Get-FileHash $src -Algorithm SHA256).Hash.Substring(0, 12)
    $dll = Join-Path $req.cache "ToastWaiter-$hash.dll"
    if (-not (Test-Path $dll)) {
        New-Item -ItemType Directory -Force -Path $req.cache | Out-Null
        $fw = [Runtime.InteropServices.RuntimeEnvironment]::GetRuntimeDirectory()
        $wm = "$env:WINDIR\System32\WinMetadata"
        # Built under a temporary name, so a hook running at the same time never loads half a file
        $tmp = "$dll.$PID.tmp"
        & (Join-Path $fw 'csc.exe') /nologo /target:library "/out:$tmp" `
            "/r:$fw\System.Runtime.dll" "/r:$fw\System.Runtime.WindowsRuntime.dll" `
            "/r:$fw\System.Runtime.InteropServices.WindowsRuntime.dll" `
            "/r:$wm\Windows.UI.winmd" "/r:$wm\Windows.Foundation.winmd" "/r:$wm\Windows.Data.winmd" `
            $src | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "csc.exe failed to build $src" }
        try { Move-Item $tmp $dll -ErrorAction Stop } catch { Remove-Item $tmp -ErrorAction SilentlyContinue }
    }
    Add-Type -Path $dll
}

$waiter = $null
if ($req.actions) {
    Get-ToastWaiter
    $waiter = New-Object ToastWaiter $toast
}
$notifier.Show($toast)

$player = $null
if ($req.sound -and $req.sound -ne 'default' -and $req.sound -ne 'none') {
    Add-Type -AssemblyName PresentationCore
    $player = New-Object System.Windows.Media.MediaPlayer
    $player.Open([Uri]$req.sound)
    $player.Play()
}

if (-not $req.actions) {
    # Stay alive until the sound has played
    if ($player) {
        $deadline = (Get-Date).AddSeconds(10)
        do {
            Start-Sleep -Milliseconds 200
            $done = $player.NaturalDuration.HasTimeSpan -and $player.Position -ge $player.NaturalDuration.TimeSpan
        } until ($done -or (Get-Date) -gt $deadline)
    }
    exit 0
}

# Clicking the toast body: bring forward the first window up the hook's process tree,
# while the click still gives this process the right to change the foreground window
function Focus-Terminal([int]$procId) {
    $shell = New-Object -ComObject WScript.Shell
    for ($i = 0; $procId -and $i -lt 16; $i++) {
        $p = Get-Process -Id $procId -ErrorAction SilentlyContinue
        if ($p -and $p.MainWindowHandle -ne [IntPtr]::Zero) { [void]$shell.AppActivate($procId); return }
        $procId = (Get-CimInstance Win32_Process -Filter "ProcessId=$procId").ParentProcessId
    }
}

$deadline = (Get-Date).AddSeconds([int]$req.wait)
while ((Get-Date) -lt $deadline) {
    if ($waiter.Wait(500)) { break }
    if ($req.parent -and -not (Get-Process -Id $req.parent -ErrorAction SilentlyContinue)) { break }
}

try { $notifier.Hide($toast) } catch { }

if ($waiter.Action) {
    $answer = @{ action = $waiter.Action }
    if ($waiter.Action -eq 'reply') { $answer.text = "$($waiter.Text)" }
    if ($waiter.Action -eq 'open') {
        try { Focus-Terminal ([int]$req.parent) } catch { }
    }
    $json = $answer | ConvertTo-Json -Compress
    [IO.File]::WriteAllText($req.response, $json, (New-Object Text.UTF8Encoding $false))
}
exit 0
