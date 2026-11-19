<#
  Reports whether the dashboard and the dev UI started by these scripts are running.
  Usage: powershell -ExecutionPolicy Bypass -File scripts/status.ps1 [-Port 3000] [-VitePort 5173]
#>
param(
    [int]$Port = 3000,
    [int]$VitePort = 5173
)

$ErrorActionPreference = 'SilentlyContinue'
$root = Split-Path -Parent $PSScriptRoot
$runDir = Join-Path $root '.dev'
$apiUrl = "http://127.0.0.1:$Port"
$uiUrl = "http://127.0.0.1:$VitePort"

$serverPid = if (Test-Path (Join-Path $runDir 'server.pid')) { Get-Content (Join-Path $runDir 'server.pid') } else { $null }
$uiPid = if (Test-Path (Join-Path $runDir 'ui.pid')) { Get-Content (Join-Path $runDir 'ui.pid') } else { $null }

try {
    $health = Invoke-WebRequest -UseBasicParsing "$apiUrl/healthz" -TimeoutSec 3
    Write-Host "API        : running  $apiUrl  ($($health.Content))"
    try {
        $auth = Invoke-RestMethod "$apiUrl/api/auth/status" -TimeoutSec 3
        if ($auth.setupRequired) {
            Write-Host 'Auth       : setup required (open /setup)'
        }
        elseif ($auth.authenticated) {
            Write-Host "Auth       : signed in as $($auth.user.username)"
        }
        else {
            Write-Host 'Auth       : signed out (open /login)'
        }
    }
    catch {
        Write-Host 'Auth       : unavailable'
    }
}
catch {
    Write-Host 'API        : stopped'
}

if ($serverPid) {
    $process = Get-Process -Id $serverPid -ErrorAction SilentlyContinue
    if ($null -ne $process) { Write-Host "Server PID : $serverPid" }
}

try {
    $ui = Invoke-WebRequest -UseBasicParsing "$uiUrl/" -TimeoutSec 3
    Write-Host "Dev UI     : running  $uiUrl  ($($ui.StatusCode))"
}
catch {
    Write-Host 'Dev UI     : stopped'
}

if ($uiPid) {
    $process = Get-Process -Id $uiPid -ErrorAction SilentlyContinue
    if ($null -ne $process) { Write-Host "UI PID     : $uiPid" }
}
