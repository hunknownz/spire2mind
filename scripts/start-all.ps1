param(
    [ValidateSet("tui", "headless-smoke", "long-soak")]
    [string]$Mode = "tui",
    [int]$Attempts = 0,
    [int]$MaxCycles = 0,
    [int]$TimeoutSeconds = 900,
    [int]$IdleTimeoutSeconds = 120,
    [string]$Language = "zh",
    [string]$FastMode = "instant",
    [string]$Planner = "mcts",
    # TTS profile: melotts-default, melotts-bright, kokoro-cute, kokoro-calm
    [string]$TTSProfile = "melotts-default",
    # Agent model preset: qwen35a3b-coding-nvfp4, qwen35a3b, qwen8b, qwen4b, claude-cli
    [string]$AgentPreset = "qwen35a3b-coding-nvfp4",
    [string]$BaseUrl = "http://192.168.3.23:11434",
    [string]$ReplaceExisting = "1"
)

$ErrorActionPreference = "Stop"
$scriptRoot = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $MyInvocation.MyCommand.Path }
$repoRoot = Split-Path -Parent $scriptRoot

$ttsUtilsPath = Join-Path $scriptRoot "tts-profile-utils.ps1"
if (Test-Path $ttsUtilsPath) {
    . $ttsUtilsPath
} else {
    Write-Warning "tts-profile-utils.ps1 not found at $ttsUtilsPath"
}

# ── helpers ──────────────────────────────────────────────────────────────────

function Stop-AllSpire2MindServices {
    $patterns = @(
        'start-spire2mind-(local-llm|claude-cli|api|all)\.ps1',
        'run-tui\.ps1',
        'headless-smoke\.ps1',
        'long-soak\.ps1',
        'go\.exe"\s+run\s+\.\\cmd\\spire2mind\s+play(?:\s|$)',
        'spire2mind\.exe\s+play(?:\s|$)',
        'tools\\tts-player\\index\.mjs',
        'kokoro_server\.py',
        'melotts_server\.py'
    )

    try {
        $all = Get-CimInstance Win32_Process | Where-Object {
            $cl = $_.CommandLine
            if (-not $cl) { return $false }
            foreach ($p in $patterns) {
                if ($cl -match $p) { return $true }
            }
            return $false
        }
    }
    catch {
        Write-Warning "Could not enumerate processes: $($_.Exception.Message)"
        return
    }

    foreach ($proc in $all) {
        if ($proc.ProcessId -eq $PID) { continue }
        try {
            Stop-Process -Id $proc.ProcessId -Force -ErrorAction Stop
            Write-Host "  Stopped PID $($proc.ProcessId) [$($proc.Name)]"
        }
        catch {
            Write-Warning "  Could not stop PID $($proc.ProcessId): $($_.Exception.Message)"
        }
    }
}

function Wait-ForHttpReady {
    param(
        [string]$Url,
        [int]$TimeoutSeconds = 30,
        [string]$Label = $Url
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $null = Invoke-RestMethod -Uri $Url -TimeoutSec 2 -ErrorAction Stop
            Write-Host "  $Label is ready."
            return $true
        }
        catch { }
        Start-Sleep -Milliseconds 500
    }
    Write-Warning "  $Label did not become ready within ${TimeoutSeconds}s — continuing anyway."
    return $false
}

function Resolve-TTSProfileForStartAll {
    param([string]$ProfileName)

    $hardcodedDefault = @{
        name             = "melotts-default"
        provider         = "melotts"
        fallbackProvider = "windows-sapi"
        baseUrl          = "http://127.0.0.1:18080"
        model            = "melotts"
        voice            = "female"
        speed            = 1.0
        streamerStyle    = "warm"
    }

    if (-not (Get-Command Resolve-TTSProfile -ErrorAction SilentlyContinue)) {
        Write-Warning "tts-profile-utils not loaded; using hardcoded melotts-default."
        return $hardcodedDefault
    }

    try {
        $p = Resolve-TTSProfile -RepoRoot $repoRoot -ProfileName $ProfileName
        if ($p -and $p.provider) { return $p }
        return $hardcodedDefault
    }
    catch {
        Write-Warning "TTS profile resolution failed: $($_.Exception.Message). Using melotts-default."
        return $hardcodedDefault
    }
}

# ── 1. kill existing instances ────────────────────────────────────────────────

if ($ReplaceExisting -match '^(1|true|yes|on)$') {
    Write-Host "==> Stopping existing Spire2Mind services..."
    Stop-AllSpire2MindServices | Out-Null
    Start-Sleep -Milliseconds 800
}

# ── 2. resolve TTS profile ────────────────────────────────────────────────────

$resolvedTTSProfile = $null
if (Get-Command Resolve-TTSProfile -ErrorAction SilentlyContinue) {
    try {
        $raw = Resolve-TTSProfile -RepoRoot $repoRoot -ProfileName $TTSProfile
        if ($raw -is [System.Array]) {
            $resolvedTTSProfile = $raw | Where-Object { $_ -is [hashtable] } | Select-Object -Last 1
        } else {
            $resolvedTTSProfile = $raw
        }
    } catch { }
}
if (-not $resolvedTTSProfile -or -not $resolvedTTSProfile.provider) {
    $resolvedTTSProfile = @{
        name             = "melotts-default"
        provider         = "melotts"
        fallbackProvider = "windows-sapi"
        baseUrl          = "http://127.0.0.1:18080"
        model            = "melotts"
        voice            = "female"
        speed            = 1.0
        streamerStyle    = "warm"
    }
}
$ttsProvider = [string]$resolvedTTSProfile.provider

Write-Host "==> TTS profile: $([string]$resolvedTTSProfile.name) (provider: $ttsProvider)"

# ── 3. start TTS server (Kokoro or MeloTTS) if needed ────────────────────────

if ($ttsProvider -eq "kokoro") {
    Write-Host "==> Starting Kokoro TTS server (port 18081)..."
    $kokoroScript = Join-Path $scriptRoot "start-local-kokoro.ps1"
    & powershell.exe -ExecutionPolicy Bypass -File $kokoroScript `
        -Voice ([string]$resolvedTTSProfile.voice) `
        -Speed ([double]$resolvedTTSProfile.speed) `
        -ReplaceExisting "0"
    Wait-ForHttpReady -Url "http://127.0.0.1:18081/health" -Label "Kokoro" -TimeoutSeconds 30 | Out-Null
}
elseif ($ttsProvider -eq "melotts") {
    Write-Host "==> Starting MeloTTS server (port 18080)..."
    $meloScript = Join-Path $scriptRoot "start-local-melotts.ps1"
    & powershell.exe -ExecutionPolicy Bypass -File $meloScript `
        -Speed ([double]$resolvedTTSProfile.speed) `
        -ReplaceExisting "0"
    Wait-ForHttpReady -Url "http://127.0.0.1:18080/health" -Label "MeloTTS" -TimeoutSeconds 30 | Out-Null
}

# ── 4. set TTS env so downstream scripts pick it up ──────────────────────────

$env:SPIRE2MIND_TTS_AUTO_SPEAK = "1"
$env:SPIRE2MIND_TTS_PROVIDER    = [string]$resolvedTTSProfile.provider
$env:SPIRE2MIND_TTS_FALLBACK_PROVIDER = [string]$resolvedTTSProfile.fallbackProvider
$env:SPIRE2MIND_TTS_BASE_URL    = [string]$resolvedTTSProfile.baseUrl
$env:SPIRE2MIND_TTS_MODEL       = [string]$resolvedTTSProfile.model
$env:SPIRE2MIND_TTS_VOICE       = [string]$resolvedTTSProfile.voice
$env:SPIRE2MIND_TTS_SPEED       = [string]$resolvedTTSProfile.speed
if ($resolvedTTSProfile.streamerStyle) {
    $env:SPIRE2MIND_STREAMER_STYLE = [string]$resolvedTTSProfile.streamerStyle
}

# ── 5. start agent ────────────────────────────────────────────────────────────

Write-Host "==> Starting agent (preset: $AgentPreset)..."

$commonArgs = @(
    "-Mode",          $Mode,
    "-Attempts",      [string]$Attempts,
    "-MaxCycles",     [string]$MaxCycles,
    "-TimeoutSeconds",[string]$TimeoutSeconds,
    "-IdleTimeoutSeconds", [string]$IdleTimeoutSeconds,
    "-Language",      $Language,
    "-FastMode",      $FastMode,
    "-Planner",       $Planner,
    "-ReplaceExisting", "0"
)

switch ($AgentPreset) {
    "qwen35a3b-coding-nvfp4" {
        $agentScript = Join-Path $scriptRoot "start-spire2mind-qwen35a3b-coding-nvfp4.ps1"
        & powershell.exe -ExecutionPolicy Bypass -File $agentScript @commonArgs -BaseUrl $BaseUrl
    }
    "qwen35a3b" {
        $agentScript = Join-Path $scriptRoot "start-spire2mind-qwen35a3b.ps1"
        & powershell.exe -ExecutionPolicy Bypass -File $agentScript @commonArgs -BaseUrl $BaseUrl
    }
    "qwen8b" {
        $agentScript = Join-Path $scriptRoot "start-spire2mind-qwen8b.ps1"
        & powershell.exe -ExecutionPolicy Bypass -File $agentScript @commonArgs -BaseUrl $BaseUrl
    }
    "qwen4b" {
        $agentScript = Join-Path $scriptRoot "start-spire2mind-qwen4b.ps1"
        & powershell.exe -ExecutionPolicy Bypass -File $agentScript @commonArgs -BaseUrl $BaseUrl
    }
    "claude-cli" {
        $agentScript = Join-Path $scriptRoot "start-spire2mind-claude-cli.ps1"
        & powershell.exe -ExecutionPolicy Bypass -File $agentScript @commonArgs
    }
    default {
        throw "Unknown agent preset '$AgentPreset'. Use: qwen35a3b-coding-nvfp4, qwen35a3b, qwen8b, qwen4b, claude-cli"
    }
}

exit $LASTEXITCODE
