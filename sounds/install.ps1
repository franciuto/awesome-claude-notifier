# Install the bundled sounds into %LOCALAPPDATA%\claude-notify\sounds as claude-notify-<group>.mp3,
# where the Windows toasts play them from.
# Usage: install.ps1 [group ...]   (default: all groups)
$ErrorActionPreference = 'Stop'

$dest = Join-Path $env:LOCALAPPDATA 'claude-notify\sounds'
New-Item -ItemType Directory -Force -Path $dest | Out-Null

$groups = if ($args.Count) { $args } else { 'needs-you', 'done', 'paused', 'error', 'limit', 'info' }

foreach ($g in $groups) {
    $target = Join-Path $dest "claude-notify-$g.mp3"
    Copy-Item (Join-Path $PSScriptRoot "mp3\$g.mp3") $target
    Write-Host "installed $target"
}
