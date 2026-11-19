# Shared helpers for the local runtime scripts. Dot-source, do not execute.

function Resolve-GoBinary {
    # Honour GO_BINARY (same variable the e2e build script uses), then PATH, then the
    # usual install locations. No user-specific paths are assumed.
    $candidates = @($env:GO_BINARY, 'go')
    if ($env:ProgramFiles) {
        $candidates += Join-Path $env:ProgramFiles 'Go\bin\go.exe'
    }
    if ($env:LOCALAPPDATA) {
        $candidates += Join-Path $env:LOCALAPPDATA 'Programs\Go\bin\go.exe'
    }
    if ($env:GOROOT) {
        $candidates += Join-Path $env:GOROOT 'bin\go.exe'
    }

    foreach ($candidate in $candidates) {
        if ([string]::IsNullOrWhiteSpace($candidate)) {
            continue
        }
        $command = Get-Command $candidate -ErrorAction SilentlyContinue
        if ($null -ne $command) {
            return $command.Source
        }
    }

    throw "Go 1.24+ was not found. Install it from https://go.dev/dl/, put it on PATH, or set GO_BINARY."
}

function Resolve-NpmCommand {
    $command = Get-Command npm -ErrorAction SilentlyContinue
    if ($null -eq $command) {
        throw "npm was not found. Install Node.js 20+ from https://nodejs.org/ and re-run."
    }
    return $command.Source
}

function Import-DotEnv {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }

    foreach ($rawLine in Get-Content -LiteralPath $Path) {
        $line = $rawLine.Trim()
        if ($line.Length -eq 0 -or $line.StartsWith('#')) {
            continue
        }
        $separator = $line.IndexOf('=')
        if ($separator -lt 1) {
            continue
        }
        $name = $line.Substring(0, $separator).Trim()
        $value = $line.Substring($separator + 1).Trim()
        if ($value.Length -ge 2 -and $value.StartsWith('"') -and $value.EndsWith('"')) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        # Real environment variables win over .env, and an empty .env entry never
        # clobbers a configured value.
        $existing = [Environment]::GetEnvironmentVariable($name, 'Process')
        if ([string]::IsNullOrWhiteSpace($existing)) {
            [Environment]::SetEnvironmentVariable($name, $value, 'Process')
        }
    }
}

function Test-PortFree {
    param([Parameter(Mandatory = $true)][int]$Port)

    $listener = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    return $null -eq $listener
}

function Assert-ClientDependencies {
    param([Parameter(Mandatory = $true)][string]$WebDir)

    if (Test-Path -LiteralPath (Join-Path $WebDir 'node_modules')) {
        return
    }

    Write-Host 'Installing client dependencies (first run)...'
    Push-Location $WebDir
    try {
        & npm ci --no-fund --no-audit
        if ($LASTEXITCODE -ne 0) { throw 'npm ci failed in web/' }
    }
    finally {
        Pop-Location
    }
}
