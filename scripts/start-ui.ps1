<#
  Starts the Vite dev client in the background, proxying /api to the running API.
  Usage: powershell -ExecutionPolicy Bypass -File scripts/start-ui.ps1 [-VitePort 5173] [-ApiPort 3000]
#>
param(
    [int]$VitePort = 5173,
    [int]$ApiPort = 3000
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot 'lib.ps1')

$webDir = Join-Path $root 'web'
$runDir = Join-Path $root '.dev'
$viteBin = Join-Path $webDir 'node_modules\vite\bin\vite.js'
$node = (Get-Command node -ErrorAction SilentlyContinue)
if ($null -eq $node) {
    throw 'Node.js was not found. Install Node.js 20+ and re-run.'
}
if (-not (Test-Path -LiteralPath $viteBin)) {
    throw 'Client dependencies are missing. Run "npm ci" in web/ first.'
}
if (-not (Test-PortFree -Port $VitePort)) {
    $existing = Get-NetTCPConnection -LocalPort $VitePort -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    throw "Port $VitePort is already in use (PID $($existing.OwningProcess))."
}

New-Item -ItemType Directory -Force -Path $runDir | Out-Null
$stdout = Join-Path $runDir 'ui.log'
$stderr = Join-Path $runDir 'ui.err.log'

$env:API_PORT = "$ApiPort"
$process = Start-Process -FilePath $node.Source `
    -ArgumentList @($viteBin, '--host', '127.0.0.1', '--port', "$VitePort", '--strictPort') `
    -WorkingDirectory $webDir `
    -WindowStyle Hidden `
    -RedirectStandardOutput $stdout `
    -RedirectStandardError $stderr `
    -PassThru
Set-Content -LiteralPath (Join-Path $runDir 'ui.pid') -Value $process.Id

$deadline = (Get-Date).AddSeconds(25)
$ready = $false
while ((Get-Date) -lt $deadline) {
    if ($process.HasExited) {
        throw "Vite exited with code $($process.ExitCode). See $stdout and $stderr."
    }
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$VitePort/" -TimeoutSec 2
        if ($response.StatusCode -eq 200) { $ready = $true; break }
    }
    catch {
        Start-Sleep -Milliseconds 300
    }
}
if (-not $ready) {
    throw "Vite did not become ready. See $stdout."
}

Write-Host ''
Write-Host "Dev UI:   http://127.0.0.1:$VitePort"
Write-Host "API:      http://127.0.0.1:$ApiPort"
Write-Host "Process:  PID $($process.Id) (detached)"
Write-Host "Logs:     $stdout"
Write-Host "Stop with scripts/stop.ps1"
