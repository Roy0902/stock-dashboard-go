<#
  Runs every required gate for the project.
  Usage: powershell -ExecutionPolicy Bypass -File scripts/verify.ps1 [-WithE2E]
#>
param(
    [switch]$WithE2E
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot 'lib.ps1')
$go = Resolve-GoBinary
$npm = Resolve-NpmCommand
$gofmt = Join-Path (Split-Path -Parent $go) $(if ($env:OS -eq 'Windows_NT') { 'gofmt.exe' } else { 'gofmt' })
if (-not (Test-Path -LiteralPath $gofmt)) {
    $gofmt = 'gofmt'
}

Push-Location (Join-Path $root 'server')
try {
    $unformatted = & $gofmt -l .
    if ($unformatted) {
        throw "gofmt found unformatted files:`n$($unformatted -join "`n")"
    }
    Write-Host 'gofmt: clean'
    & $go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
    Write-Host 'go vet: clean'
    & $go test ./... -count=1
    if ($LASTEXITCODE -ne 0) { throw 'go test failed' }
    Write-Host 'go test: passed'
}
finally {
    Pop-Location
}

Push-Location (Join-Path $root 'web')
try {
    foreach ($script in @('lint', 'typecheck', 'format:check', 'test', 'build')) {
        Write-Host "npm run $script"
        & $npm run $script
        if ($LASTEXITCODE -ne 0) { throw "npm run $script failed" }
    }
}
finally {
    Pop-Location
}

if ($WithE2E) {
    Push-Location (Join-Path $root 'e2e')
    try {
        Write-Host 'npm run typecheck'
        & $npm run typecheck
        if ($LASTEXITCODE -ne 0) { throw 'e2e typecheck failed' }

        Write-Host 'npm test (acceptance)'
        & $npm test
        if ($LASTEXITCODE -ne 0) { throw 'acceptance tests failed' }
    }
    finally {
        Pop-Location
    }
}

Write-Host 'All gates passed.'
