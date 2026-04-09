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
    [string]$ApiBaseUrl = "",
    [string]$ApiKey = "",
    [string]$ApiProvider = "",
    [ValidateSet("tools", "structured", "auto", "")]
    [string]$ApiDecisionMode = "",
    [string]$Model = "",
    [string]$ForceModelEval = "",
    [string]$ReplaceExisting = "1"
)

$ErrorActionPreference = "Stop"

$scriptRoot = $PSScriptRoot
. (Join-Path $PSScriptRoot "tts-profile-utils.ps1")
$resolvedForceModelEval = $ForceModelEval -match '^(1|true|yes|on)$'
$resolvedReplaceExisting = $ReplaceExisting -match '^(1|true|yes|on)$'
$resolvedTTSProfile = Resolve-TTSProfile -RepoRoot (Split-Path -Parent $scriptRoot)

function Start-VisibleTuiProcess {
    param(
        [string]$RunTuiScript,
        [int]$Attempts,
        [int]$MaxCycles,
        [string]$Language,
        [string]$Provider,
        [string]$FastMode,
        [string]$Planner,
        [bool]$ForceModelEval
    )

    $powershellArgs = @(
        "powershell.exe",
        "-NoExit",
        "-ExecutionPolicy",
        "Bypass",
        "-File",
        $RunTuiScript,
        "-Attempts",
        [string]$Attempts,
        "-MaxCycles",
        [string]$MaxCycles,
        "-Language",
        $Language,
        "-Provider",
        $Provider,
        "-FastMode",
        $FastMode,
        "-Planner",
        $Planner
    )

    if ($ForceModelEval) {
        $powershellArgs += "-ForceModelEval"
    }

    $useWindowsTerminal = $env:SPIRE2MIND_TUI_USE_WINDOWS_TERMINAL -match '^(1|true|yes|on)$'
    $wt = Get-Command wt.exe -ErrorAction SilentlyContinue
    if ($useWindowsTerminal -and $wt) {
        $wtArgs = @(
            "new-tab",
            "--title",
            "Spire2Mind TUI"
        ) + $powershellArgs
        Start-Process -FilePath $wt.Source -ArgumentList $wtArgs | Out-Null
        return
    }

    Start-Process -FilePath "powershell.exe" -ArgumentList $powershellArgs[1..($powershellArgs.Length - 1)] | Out-Null
}

function Start-TTSPlayerIfEnabled {
    param(
        [string]$ScriptRoot
    )

    if ($env:SPIRE2MIND_TTS_AUTO_SPEAK -notmatch '^(1|true|yes|on)$') {
        return
    }

    $launcher = Join-Path $ScriptRoot "start-tts-player.ps1"
    if (-not (Test-Path $launcher)) {
        Write-Warning "TTS player launcher not found: $launcher"
        return
    }

    $ttsArgs = @(
        "-ExecutionPolicy", "Bypass",
        "-File", $launcher,
        "-ReplaceExisting"
    )
    if ($env:SPIRE2MIND_TTS_PROVIDER) { $ttsArgs += @("-Provider", $env:SPIRE2MIND_TTS_PROVIDER) }
    if ($env:SPIRE2MIND_TTS_FALLBACK_PROVIDER) { $ttsArgs += @("-FallbackProvider", $env:SPIRE2MIND_TTS_FALLBACK_PROVIDER) }
    if ($env:SPIRE2MIND_TTS_BASE_URL) { $ttsArgs += @("-BaseUrl", $env:SPIRE2MIND_TTS_BASE_URL) }
    if ($env:SPIRE2MIND_TTS_API_KEY) { $ttsArgs += @("-ApiKey", $env:SPIRE2MIND_TTS_API_KEY) }
    if ($env:SPIRE2MIND_TTS_MODEL) { $ttsArgs += @("-Model", $env:SPIRE2MIND_TTS_MODEL) }
    if ($env:SPIRE2MIND_TTS_VOICE) { $ttsArgs += @("-Voice", $env:SPIRE2MIND_TTS_VOICE) }
    if ($env:SPIRE2MIND_TTS_SPEED) { $ttsArgs += @("-Speed", $env:SPIRE2MIND_TTS_SPEED) }

    & powershell.exe @ttsArgs
}

function Stop-ExistingSpire2MindInstances {
    param(
        [int]$CurrentPid
    )

    try {
        $processes = Get-CimInstance Win32_Process | Where-Object {
            $_.CommandLine -and (
                $_.CommandLine -match 'start-spire2mind-(claude-cli|api)\.ps1' -or
                $_.CommandLine -match 'run-tui\.ps1' -or
                $_.CommandLine -match 'headless-smoke\.ps1' -or
                $_.CommandLine -match 'long-soak\.ps1' -or
                $_.CommandLine -match 'go\.exe"\s+run\s+\.\\cmd\\spire2mind\s+play(?:\s|$)' -or
                $_.CommandLine -match 'spire2mind\.exe\s+play(?:\s|$)'
            )
        }
    }
    catch {
        Write-Warning "Unable to inspect existing Spire2Mind processes; skipping cleanup: $($_.Exception.Message)"
        return
    }

    foreach ($process in $processes) {
        if ($process.ProcessId -eq $CurrentPid) {
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

function Test-TrustedLocalApiBaseUrl {
    param(
        [string]$Url
    )

    if ([string]::IsNullOrWhiteSpace($Url)) {
        return $false
    }

    try {
        $uri = [System.Uri]$Url
    }
    catch {
        return $false
    }

    $targetHost = $uri.Host
    if ([string]::IsNullOrWhiteSpace($targetHost)) {
        return $false
    }
    if ($targetHost -ieq "localhost") {
        return $true
    }

    $ip = $null
    if (-not [System.Net.IPAddress]::TryParse($targetHost, [ref]$ip)) {
        return $false
    }
    if ($ip.IPAddressToString -eq "127.0.0.1") {
        return $true
    }

    $bytes = $ip.GetAddressBytes()
    if ($bytes.Length -ne 4) {
        return $false
    }
    if ($bytes[0] -eq 10) {
        return $true
    }
    if ($bytes[0] -eq 192 -and $bytes[1] -eq 168) {
        return $true
    }
    if ($bytes[0] -eq 172 -and $bytes[1] -ge 16 -and $bytes[1] -le 31) {
        return $true
    }
    return $false
}

if ($resolvedReplaceExisting) {
    Stop-ExistingSpire2MindInstances -CurrentPid $PID
}

$resolvedBaseUrl = if ([string]::IsNullOrWhiteSpace($ApiBaseUrl)) {
    if ($env:SPIRE2MIND_API_BASE_URL) {
        $env:SPIRE2MIND_API_BASE_URL
    }
    elseif ($env:ANTHROPIC_BASE_URL) {
        $env:ANTHROPIC_BASE_URL
    }
    else {
        ""
    }
} else {
    $ApiBaseUrl
}

$resolvedApiKey = if ([string]::IsNullOrWhiteSpace($ApiKey)) {
    if ($env:SPIRE2MIND_API_KEY) {
        $env:SPIRE2MIND_API_KEY
    }
    elseif ($env:ANTHROPIC_AUTH_TOKEN) {
        $env:ANTHROPIC_AUTH_TOKEN
    }
    else {
        ""
    }
} else {
    $ApiKey
}

$resolvedApiProvider = if ([string]::IsNullOrWhiteSpace($ApiProvider)) {
    if ($env:SPIRE2MIND_API_PROVIDER) {
        $env:SPIRE2MIND_API_PROVIDER
    }
    elseif ($env:ANTHROPIC_PROVIDER) {
        $env:ANTHROPIC_PROVIDER
    }
    else {
        ""
    }
} else {
    $ApiProvider
}

$resolvedModel = if ([string]::IsNullOrWhiteSpace($Model)) {
    if ($env:SPIRE2MIND_MODEL) {
        $env:SPIRE2MIND_MODEL
    }
    elseif ($env:ANTHROPIC_MODEL) {
        $env:ANTHROPIC_MODEL
    }
    else {
        "claude-sonnet-4-6"
    }
} else {
    $Model
}

$resolvedApiDecisionMode = if ([string]::IsNullOrWhiteSpace($ApiDecisionMode)) {
    if ($env:SPIRE2MIND_API_DECISION_MODE) {
        $env:SPIRE2MIND_API_DECISION_MODE
    }
    elseif ($env:SPIRE2MIND_DECISION_MODE) {
        $env:SPIRE2MIND_DECISION_MODE
    }
    else {
        ""
    }
} else {
    $ApiDecisionMode
}

if ([string]::IsNullOrWhiteSpace($resolvedBaseUrl)) {
    throw "API provider requires SPIRE2MIND_API_BASE_URL or ANTHROPIC_BASE_URL."
}
if ([string]::IsNullOrWhiteSpace($resolvedApiKey) -and -not (Test-TrustedLocalApiBaseUrl -Url $resolvedBaseUrl)) {
    throw "API provider requires SPIRE2MIND_API_KEY or ANTHROPIC_AUTH_TOKEN."
}

$env:SPIRE2MIND_MODEL_PROVIDER = "api"
$env:SPIRE2MIND_API_BASE_URL = $resolvedBaseUrl
$env:SPIRE2MIND_API_KEY = $resolvedApiKey
$env:SPIRE2MIND_API_PROVIDER = $resolvedApiProvider
if (-not [string]::IsNullOrWhiteSpace($resolvedApiDecisionMode)) {
    $env:SPIRE2MIND_API_DECISION_MODE = $resolvedApiDecisionMode
}
$env:SPIRE2MIND_MODEL = $resolvedModel
$env:SPIRE2MIND_FORCE_MODEL_EVAL = if ($resolvedForceModelEval) { "1" } else { "0" }
if (-not $env:SPIRE2MIND_STREAMER_STYLE -and $resolvedTTSProfile.streamerStyle) {
    $env:SPIRE2MIND_STREAMER_STYLE = [string]$resolvedTTSProfile.streamerStyle
}

switch ($Mode) {
    "tui" {
        Start-TTSPlayerIfEnabled -ScriptRoot $scriptRoot
        Start-VisibleTuiProcess `
            -RunTuiScript (Join-Path $scriptRoot "run-tui.ps1") `
            -Attempts $Attempts `
            -MaxCycles $MaxCycles `
            -Language $Language `
            -Provider "api" `
            -FastMode $FastMode `
            -Planner $Planner `
            -ForceModelEval $resolvedForceModelEval
    }
    "headless-smoke" {
        & (Join-Path $scriptRoot "headless-smoke.ps1") `
            -TimeoutSeconds $TimeoutSeconds `
            -IdleTimeoutSeconds $IdleTimeoutSeconds `
            -Attempts $Attempts `
            -FastMode $FastMode `
            -Planner $Planner
    }
    "long-soak" {
        & (Join-Path $scriptRoot "long-soak.ps1") `
            -Attempts $Attempts `
            -TimeoutSeconds $TimeoutSeconds `
            -IdleTimeoutSeconds $IdleTimeoutSeconds `
            -FastMode $FastMode `
            -Planner $Planner
    }
}

exit $LASTEXITCODE
