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
    [string]$BaseUrl = "http://192.168.3.23:11434",
    [string]$Model = "qwen3.5:35b-a3b",
    [string]$FallbackModel = "qwen3:8b",
    [int]$ModelContext = 8192,
    [ValidateSet("tools", "structured", "auto")]
    [string]$ApiDecisionMode = "structured",
    [switch]$ForceModelEval,
    [switch]$PullModel,
    [string]$ReplaceExisting = "1"
)

$ErrorActionPreference = "Stop"

$scriptRoot = $PSScriptRoot
$repoRoot = Split-Path -Parent $scriptRoot
function Get-Spire2MindManagedProcesses {
    try {
        return Get-CimInstance Win32_Process | Where-Object {
            $_.CommandLine -and (
                $_.CommandLine -match 'start-spire2mind-(local-llm|claude-cli|api)\.ps1' -or
                $_.CommandLine -match 'run-tui\.ps1' -or
                $_.CommandLine -match 'headless-smoke\.ps1' -or
                $_.CommandLine -match 'long-soak\.ps1' -or
                $_.CommandLine -match 'go\.exe"\s+run\s+\.\\cmd\\spire2mind\s+play(?:\s|$)' -or
                $_.CommandLine -match 'spire2mind\.exe\s+play(?:\s|$)'
            )
        }
    }
    catch {
        return @()
    }
}

function Stop-ExistingSpire2MindInstances {
    param(
        [int]$CurrentPid
    )

    $processes = Get-Spire2MindManagedProcesses
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

function Wait-ForSingleAgentInstance {
    param(
        [int]$TimeoutSeconds = 90
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $agents = @(Get-Spire2MindManagedProcesses | Where-Object {
            $_.CommandLine -match 'go\.exe"\s+run\s+\.\\cmd\\spire2mind\s+play(?:\s|$)'
        })
        if ($agents.Count -eq 1) {
            return $agents[0]
        }
        Start-Sleep -Milliseconds 500
    }

    $agents = @(Get-Spire2MindManagedProcesses | Where-Object {
        $_.CommandLine -match 'go\.exe"\s+run\s+\.\\cmd\\spire2mind\s+play(?:\s|$)'
    })
    if ($agents.Count -eq 0) {
        throw "Local LLM launch did not start an agent instance."
    }
    throw "Expected exactly one local LLM agent instance, found $($agents.Count)."
}

function Test-OllamaReady {
    param(
        [string]$Url
    )

    try {
        $null = Invoke-RestMethod -Uri ($Url.TrimEnd('/') + "/api/tags") -TimeoutSec 3
        return $true
    }
    catch {
        return $false
    }
}

function Test-TrustedLocalBaseUrl {
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

function Test-LoopbackBaseUrl {
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

    return $uri.Host -ieq "localhost" -or $uri.Host -eq "127.0.0.1"
}

function Ensure-OllamaReady {
    param(
        [string]$ExecutablePath,
        [string]$Url
    )

    if (Test-OllamaReady -Url $Url) {
        return
    }

    Start-Process -FilePath $ExecutablePath -ArgumentList "serve" -WindowStyle Hidden | Out-Null

    $deadline = (Get-Date).AddSeconds(20)
    while ((Get-Date) -lt $deadline) {
        if (Test-OllamaReady -Url $Url) {
            return
        }
        Start-Sleep -Milliseconds 500
    }

    throw "Ollama did not become ready at $Url within 20 seconds."
}

function Ensure-ModelPresent {
    param(
        [string]$ExecutablePath,
        [string]$Name
    )

    if ([string]::IsNullOrWhiteSpace($Name)) {
        return
    }

    $models = & $ExecutablePath list
    if ($models -match "(?m)^\Q$Name\E\s") {
        return
    }

    & $ExecutablePath pull $Name
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to pull model $Name"
    }
}

if (-not (Test-TrustedLocalBaseUrl -Url $BaseUrl)) {
    throw "Local LLM base URL must be a trusted local/LAN address."
}

$isLoopbackBaseUrl = Test-LoopbackBaseUrl -Url $BaseUrl
$ollamaExe = $null
if ($isLoopbackBaseUrl) {
    $ollamaExe = @(
        "$env:LOCALAPPDATA\Programs\Ollama\ollama.exe",
        "$env:ProgramFiles\Ollama\ollama.exe"
    ) | Where-Object { Test-Path $_ } | Select-Object -First 1

    if (-not $ollamaExe) {
        throw "Ollama is not installed. Install it first, or point BaseUrl to a reachable local/LAN model endpoint."
    }

    Ensure-OllamaReady -ExecutablePath $ollamaExe -Url $BaseUrl
}

if ($PullModel) {
    if (-not $isLoopbackBaseUrl) {
        throw "PullModel is only supported for loopback Ollama endpoints."
    }
    Ensure-ModelPresent -ExecutablePath $ollamaExe -Name $Model
    Ensure-ModelPresent -ExecutablePath $ollamaExe -Name $FallbackModel
}

$headlessSmokeScript = Join-Path $scriptRoot "headless-smoke.ps1"
$longSoakScript = Join-Path $scriptRoot "long-soak.ps1"
$apiStartScript = Join-Path $scriptRoot "start-spire2mind-api.ps1"

if ($ReplaceExisting -match '^(1|true|yes|on)$') {
    Stop-ExistingSpire2MindInstances -CurrentPid $PID
}

$env:SPIRE2MIND_MODEL_PROVIDER = "api"
$env:SPIRE2MIND_API_PROVIDER = "openai"
$env:SPIRE2MIND_API_BASE_URL = $BaseUrl
$env:SPIRE2MIND_API_KEY = ""
$env:SPIRE2MIND_API_DECISION_MODE = $ApiDecisionMode
$env:SPIRE2MIND_MODEL = $Model
$env:SPIRE2MIND_MODEL_CONTEXT = [string]$ModelContext
$env:SPIRE2MIND_LOCAL_FALLBACK_MODEL = $FallbackModel
$env:SPIRE2MIND_FORCE_MODEL_EVAL = if ($ForceModelEval) { "1" } else { "0" }

switch ($Mode) {
    "tui" {
        & $apiStartScript `
            -Mode "tui" `
            -Attempts $Attempts `
            -MaxCycles $MaxCycles `
            -Language $Language `
            -FastMode $FastMode `
            -Planner $Planner `
            -ApiBaseUrl $BaseUrl `
            -ApiProvider "openai" `
            -ApiDecisionMode $ApiDecisionMode `
            -Model $Model `
            -ForceModelEval $ForceModelEval.IsPresent `
            -ReplaceExisting 0

        $agent = Wait-ForSingleAgentInstance -TimeoutSeconds 20
        Write-Host "Local LLM TUI started with one agent instance: PID $($agent.ProcessId)"
    }
    "headless-smoke" {
        & $headlessSmokeScript `
            -TimeoutSeconds $TimeoutSeconds `
            -IdleTimeoutSeconds $IdleTimeoutSeconds `
            -Attempts $Attempts `
            -FastMode $FastMode `
            -Planner $Planner
    }
    "long-soak" {
        & $longSoakScript `
            -Attempts $Attempts `
            -TimeoutSeconds $TimeoutSeconds `
            -IdleTimeoutSeconds $IdleTimeoutSeconds `
            -FastMode $FastMode `
            -Planner $Planner
    }
}

exit $LASTEXITCODE
