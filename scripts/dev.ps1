<#
  Runs the Go API and the Vite dev server together. Ctrl+C stops both.
  Usage:
    powershell -ExecutionPolicy Bypass -File scripts/dev.ps1
    powershell -ExecutionPolicy Bypass -File scripts/dev.ps1 -ApiPort 3000 -VitePort 5173
#>
param(
    [int]$ApiPort = 3000,
    [int]$VitePort = 5173,
    [string]$DataDir = ''
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot 'lib.ps1')

Import-DotEnv -Path (Join-Path $root '.env')
$go = Resolve-GoBinary
$npm = Resolve-NpmCommand
$webDir = Join-Path $root 'web'
$serverDir = Join-Path $root 'server'
$runDir = Join-Path $root '.dev'
$binary = Join-Path $runDir 'dashboard-dev.exe'
$log = Join-Path $runDir 'api.log'

Assert-ClientDependencies -WebDir $webDir

foreach ($candidate in @($ApiPort, $VitePort)) {
    if (-not (Test-PortFree -Port $candidate)) {
        throw "Port $candidate is already in use."
    }
}

if ([string]::IsNullOrWhiteSpace($DataDir)) {
    $DataDir = Join-Path $root 'data'
}
New-Item -ItemType Directory -Force -Path $DataDir, $runDir | Out-Null

Write-Host 'Building the API binary...'
Push-Location $serverDir
try {
    & $go build -o $binary ./cmd/dashboard
    if ($LASTEXITCODE -ne 0) { throw 'server build failed' }
}
finally {
    Pop-Location
}

$env:PORT = "$ApiPort"
$env:DATA_DIR = $DataDir
$env:API_PORT = "$ApiPort"

$api = Start-Process -FilePath $binary `
    -WorkingDirectory $serverDir `
    -WindowStyle Hidden `
    -RedirectStandardOutput $log `
    -RedirectStandardError "$log.err" `
    -PassThru

try {
    $deadline = (Get-Date).AddSeconds(15)
    $ready = $false
    while ((Get-Date) -lt $deadline) {
        if ($api.HasExited) {
            throw "API exited with code $($api.ExitCode). See $log and $log.err."
        }
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$ApiPort/healthz" -TimeoutSec 2
            if ($response.StatusCode -eq 200) { $ready = $true; break }
        }
        catch {
            Start-Sleep -Milliseconds 250
        }
    }
    if (-not $ready) {
        throw "API did not become healthy. See $log."
    }

    Write-Host ''
    Write-Host "API:  http://127.0.0.1:$ApiPort"
    Write-Host "App:  http://127.0.0.1:$VitePort"
    Write-Host "Logs: $log"
    Write-Host 'Press Ctrl+C to stop both.'
    Write-Host ''

    Push-Location $webDir
    try {
        & $npm run dev -- --host 127.0.0.1 --port $VitePort --strictPort
    }
    finally {
        Pop-Location
    }
}
finally {
    if (-not $api.HasExited) {
        Stop-Process -Id $api.Id -Force -ErrorAction SilentlyContinue
    }
}
