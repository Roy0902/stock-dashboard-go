<#
  Stops the dashboard and the dev UI started by the scripts in this folder.
  Usage: powershell -ExecutionPolicy Bypass -File scripts/stop.ps1 [-Port 3000] [-VitePort 5173]
#>
param(
    [int]$Port = 3000,
    [int]$VitePort = 5173
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$runDir = Join-Path $root '.dev'

function Stop-FromPidFile {
    param(
        [Parameter(Mandatory = $true)][string]$PidFile,
        [Parameter(Mandatory = $true)][string]$ExpectedName,
        [Parameter(Mandatory = $true)][int]$FallbackPort
    )

    $targets = @()
    if (Test-Path -LiteralPath $PidFile) {
        $recorded = Get-Content -LiteralPath $PidFile -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($recorded -match '^\d+$') {
            $targets += [int]$recorded
        }
    }
    Get-NetTCPConnection -LocalPort $FallbackPort -State Listen -ErrorAction SilentlyContinue |
        ForEach-Object { $targets += [int]$_.OwningProcess }

    $stopped = $false
    foreach ($targetId in ($targets | Select-Object -Unique)) {
        $process = Get-Process -Id $targetId -ErrorAction SilentlyContinue
        if ($null -eq $process) {
            continue
        }
        if ($process.ProcessName -ne $ExpectedName) {
            Write-Warning "Port $FallbackPort is owned by '$($process.ProcessName)' (PID $targetId); not stopping it."
            continue
        }
        Stop-Process -Id $targetId -Force
        Write-Host "Stopped $ExpectedName (PID $targetId)."
        $stopped = $true
    }
    Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
    return $stopped
}

$serverStopped = Stop-FromPidFile -PidFile (Join-Path $runDir 'server.pid') -ExpectedName 'dashboard' -FallbackPort $Port
$uiStopped = Stop-FromPidFile -PidFile (Join-Path $runDir 'ui.pid') -ExpectedName 'node' -FallbackPort $VitePort

if (-not $serverStopped -and -not $uiStopped) {
    Write-Host 'Nothing was running.'
}
