param(
    [int]$Attempts = 3,
    [int]$MaxCycles = -1,
    [string]$Language = "en",
    [string]$Provider = "claude-cli",
    [string]$FastMode = "instant",
    [string]$Planner = "mcts",
    [switch]$ForceModelEval,
    [switch]$ReplaceExisting = $true
)

$ErrorActionPreference = "Stop"

function Set-ConsoleUtf8 {
    try {
        & cmd /c chcp 65001 > $null
    }
    catch {
    }

    try {
        $utf8 = [System.Text.UTF8Encoding]::new($false)
        [Console]::InputEncoding = $utf8
        [Console]::OutputEncoding = $utf8
        $global:OutputEncoding = $utf8
    }
    catch {
    }
}

function Resolve-TuiLanguage {
    param(
        [string]$RequestedLanguage
    )

    if ([string]::IsNullOrWhiteSpace($RequestedLanguage)) {
        $requested = "en"
    }
    else {
        $requested = $RequestedLanguage.Trim().ToLowerInvariant()
    }
    $disableConsoleSafe = $env:SPIRE2MIND_TUI_DISABLE_CONSOLE_SAFE_I18N
    $consoleSafeEnabled = -not ($disableConsoleSafe -match '^(1|true|yes|on)$')

    if ($consoleSafeEnabled -and $requested -ne "en") {
        return "en"
    }

    return $requested
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$preferredGo = Join-Path $repoRoot ".tools\go-go1.26.1 go1.25.8\go\bin\go.exe"
$goExe = if (Test-Path $preferredGo) { $preferredGo } else { "go" }
$bridgeUrl = if ($env:SPIRE2MIND_BRIDGE_URL) { $env:SPIRE2MIND_BRIDGE_URL.TrimEnd("/") } else { "http://127.0.0.1:8080" }
$gameDir = if ($env:STS2_GAME_DIR) { $env:STS2_GAME_DIR } else { "C:\Program Files (x86)\Steam\steamapps\common\Slay the Spire 2" }
$gameExe = Join-Path $gameDir "SlayTheSpire2.exe"

$cacheRoot = Join-Path $repoRoot ".cache"
$goCache = Join-Path $cacheRoot "gocache"
$goModCache = Join-Path $cacheRoot "gomodcache"

New-Item -ItemType Directory -Force -Path $cacheRoot | Out-Null
New-Item -ItemType Directory -Force -Path $goCache | Out-Null
New-Item -ItemType Directory -Force -Path $goModCache | Out-Null

Set-Location $repoRoot
Set-ConsoleUtf8

$effectiveLanguage = Resolve-TuiLanguage -RequestedLanguage $Language
$resolvedForceModelEval = $ForceModelEval.IsPresent -or ($env:SPIRE2MIND_FORCE_MODEL_EVAL -match '^(1|true|yes|on)$')

$env:GOCACHE = $goCache
$env:GOMODCACHE = $goModCache
$env:GOPROXY = if ($env:GOPROXY) { $env:GOPROXY } else { "https://proxy.golang.org,direct" }
$env:SPIRE2MIND_MODEL_PROVIDER = $Provider
$env:SPIRE2MIND_LANGUAGE = $effectiveLanguage
$env:SPIRE2MIND_MAX_ATTEMPTS = [string]$Attempts
$env:SPIRE2MIND_MAX_CYCLES = [string]$MaxCycles
$env:SPIRE2MIND_GAME_FAST_MODE = $FastMode
$env:SPIRE2MIND_COMBAT_PLANNER = $Planner
$env:SPIRE2MIND_FORCE_MODEL_EVAL = if ($resolvedForceModelEval) { "1" } else { "0" }

$attemptLabel = if ($Attempts -le 0) { "continuous" } else { [string]$Attempts }

function Test-BridgeReady {
    param(
        [string]$Url
    )

    try {
        $response = Invoke-RestMethod -Uri "$Url/health" -TimeoutSec 3
        if ($null -eq $response) {
            return $false
        }
        if ($response.ready -eq $true) {
            return $true
        }
        if ($response.ok -eq $true -and $null -ne $response.data -and $response.data.ready -eq $true) {
            return $true
        }
        return $false
    }
    catch {
        return $false
    }
}

function Ensure-GameAndBridgeReady {
    param(
        [string]$ExecutablePath,
        [string]$Url,
        [int]$TimeoutSec
    )

    if (-not (Get-Process -Name "SlayTheSpire2" -ErrorAction SilentlyContinue | Select-Object -First 1)) {
        if (-not (Test-Path $ExecutablePath)) {
            throw "Game executable not found at $ExecutablePath"
        }
        Start-Process -FilePath $ExecutablePath | Out-Null
    }

    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if (Test-BridgeReady -Url $Url) {
            return
        }
        Start-Sleep -Seconds 2
    }

    throw "Bridge did not become ready within $TimeoutSec seconds."
}

function Stop-ExistingSpire2MindTUI {
    param(
        [int]$CurrentPid
    )

    try {
        $processes = Get-CimInstance Win32_Process | Where-Object {
            $_.CommandLine -and (
                $_.CommandLine -match 'run-tui\.ps1' -or
                $_.CommandLine -match 'go\.exe"\s+run\s+\.\\cmd\\spire2mind\s+play(?:\s|$)'
            )
        }
    }
    catch {
        Write-Warning "Unable to inspect existing Spire2Mind processes; skipping pre-launch cleanup: $($_.Exception.Message)"
        return
    }

    foreach ($process in $processes) {
        if ($process.ProcessId -eq $CurrentPid) {
            continue
        }

        $commandLine = [string]$process.CommandLine
        if ($commandLine -match '--headless') {
            continue
        }

        try {
            Stop-Process -Id $process.ProcessId -Force -ErrorAction Stop
            Write-Host "Stopped existing Spire2Mind process: $($process.ProcessId) [$($process.Name)]"
        }
        catch {
            Write-Warning "Failed to stop existing process $($process.ProcessId): $($_.Exception.Message)"
        }
    }
}

if ($Host -and $Host.UI -and $Host.UI.RawUI) {
    $Host.UI.RawUI.WindowTitle = "Spire2Mind TUI ($attemptLabel)"
}

Write-Host "Repo: $repoRoot"
Write-Host "Go: $goExe"
Write-Host "Provider: $Provider"
Write-Host "Language: $effectiveLanguage"
if ($effectiveLanguage -ne $Language) {
    Write-Host "Requested language: $Language"
    Write-Host "Console-safe i18n: forcing English for the visible TUI"
}
Write-Host "Attempts: $attemptLabel"
if ($MaxCycles -eq 0) {
    Write-Host "Max cycles: unlimited"
}
elseif ($MaxCycles -gt 0) {
    Write-Host "Max cycles: $MaxCycles"
}
else {
    Write-Host "Max cycles: auto"
}
Write-Host "Fast mode: $FastMode"
Write-Host "Planner: $Planner"
Write-Host "Force model eval: $resolvedForceModelEval"
Write-Host "Bridge URL: $bridgeUrl"
Write-Host "Game executable: $gameExe"
Write-Host ""

if ($ReplaceExisting) {
    Stop-ExistingSpire2MindTUI -CurrentPid $PID
}

Ensure-GameAndBridgeReady -ExecutablePath $gameExe -Url $bridgeUrl -TimeoutSec 120

if ($MaxCycles -ge 0) {
    & $goExe run .\cmd\spire2mind play --attempts $Attempts --max-cycles $MaxCycles
}
else {
    & $goExe run .\cmd\spire2mind play --attempts $Attempts
}
exit $LASTEXITCODE
