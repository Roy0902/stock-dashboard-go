<#
  Builds the client and server, then runs the production server.
  Usage:
    powershell -ExecutionPolicy Bypass -File scripts/start.ps1
    powershell -ExecutionPolicy Bypass -File scripts/start.ps1 -Detached
    powershell -ExecutionPolicy Bypass -File scripts/start.ps1 -Restart -Detached
    powershell -ExecutionPolicy Bypass -File scripts/start.ps1 -Port 3001 -SkipBuild
#>
param(
    [int]$Port = 3000,
    [string]$DataDir = '',
    [switch]$SkipBuild,
    [switch]$Detached,
    [switch]$Restart
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot 'lib.ps1')

$healthUrl = "http://127.0.0.1:$Port/healthz"

function Test-DashboardHealth {
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $healthUrl -TimeoutSec 2
        return $response.StatusCode -eq 200 -and $response.Content -match '"status"\s*:\s*"ok"'
    }
    catch {
        return $false
    }
}

if (Test-DashboardHealth) {
    if (-not $Restart) {
        Write-Host "The dashboard is already running at http://127.0.0.1:$Port"
        Write-Host 'Use -Restart to replace it.'
        exit 0
    }
    & (Join-Path $PSScriptRoot 'stop.ps1') -Port $Port
    $deadline = (Get-Date).AddSeconds(10)
    while (-not (Test-PortFree -Port $Port) -and (Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 200
    }
}

if (-not (Test-PortFree -Port $Port)) {
    $owner = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
        Select-Object -First 1
    throw "Port $Port is already in use by another process (PID $($owner.OwningProcess)). Stop it or pass -Port <n>."
}

Import-DotEnv -Path (Join-Path $root '.env')
$go = Resolve-GoBinary
$npm = Resolve-NpmCommand
$webDir = Join-Path $root 'web'
$serverDir = Join-Path $root 'server'
$binary = Join-Path $serverDir 'bin\dashboard.exe'

if (-not $SkipBuild) {
    Assert-ClientDependencies -WebDir $webDir

    Write-Host 'Building the web client...'
    Push-Location $webDir
    try {
        & $npm run build
        if ($LASTEXITCODE -ne 0) { throw 'web build failed' }
    }
    finally {
        Pop-Location
    }

    Write-Host 'Building the server...'
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $binary) | Out-Null
    Push-Location $serverDir
    try {
        & $go build -o $binary ./cmd/dashboard
        if ($LASTEXITCODE -ne 0) { throw 'server build failed' }
    }
    finally {
        Pop-Location
    }
}
elseif (-not (Test-Path -LiteralPath $binary)) {
    throw "No server binary at $binary. Run once without -SkipBuild."
}

if ([string]::IsNullOrWhiteSpace($DataDir)) {
    $DataDir = Join-Path $root 'data'
}
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null

$env:PORT = "$Port"
$env:DATA_DIR = $DataDir

$missing = @()
if ([string]::IsNullOrWhiteSpace($env:FINNHUB_API_KEY)) { $missing += 'FINNHUB_API_KEY' }
if ([string]::IsNullOrWhiteSpace($env:TWELVEDATA_API_KEY)) { $missing += 'TWELVEDATA_API_KEY' }
if ($missing.Count -gt 0) {
    Write-Warning "Not set: $($missing -join ', '). Quotes and charts will report a missing key. Put them in .env or the environment."
}

if ($Detached) {
    $runDir = Join-Path $root '.dev'
    New-Item -ItemType Directory -Force -Path $runDir | Out-Null
    $stdout = Join-Path $runDir 'server.log'
    $stderr = Join-Path $runDir 'server.err.log'
    $process = Start-Process -FilePath $binary `
        -WorkingDirectory $serverDir `
        -WindowStyle Hidden `
        -RedirectStandardOutput $stdout `
        -RedirectStandardError $stderr `
        -PassThru
    Set-Content -LiteralPath (Join-Path $runDir 'server.pid') -Value $process.Id

    $deadline = (Get-Date).AddSeconds(20)
    $ready = $false
    while ((Get-Date) -lt $deadline) {
        if ($process.HasExited) {
            throw "The dashboard exited with code $($process.ExitCode). See $stdout and $stderr."
        }
        if (Test-DashboardHealth) { $ready = $true; break }
        Start-Sleep -Milliseconds 250
    }
    if (-not $ready) {
        throw "The dashboard did not become healthy. See $stdout."
    }

    Write-Host ''
    Write-Host "Stock dashboard: http://127.0.0.1:$Port"
    Write-Host "Data directory:  $DataDir"
    Write-Host "Process:         PID $($process.Id) (detached)"
    Write-Host "Logs:            $stdout"
    Write-Host "Stop with:       powershell -ExecutionPolicy Bypass -File scripts/stop.ps1 -Port $Port"
    exit 0
}

Write-Host ''
Write-Host "Stock dashboard: http://127.0.0.1:$Port"
Write-Host "Data directory:  $DataDir"
Write-Host 'Press Ctrl+C to stop.'
Write-Host ''

& $binary
exit $LASTEXITCODE
