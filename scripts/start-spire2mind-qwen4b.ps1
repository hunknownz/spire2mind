param(
    [ValidateSet("tui", "headless-smoke", "long-soak")]
    [string]$Mode = "tui",
    [int]$Attempts = 0,
    [int]$MaxCycles = 0,
    [int]$TimeoutSeconds = 900,
    [int]$IdleTimeoutSeconds = 120,
    [string]$Language = "en",
    [string]$FastMode = "instant",
    [string]$Planner = "mcts",
    [string]$BaseUrl = "http://127.0.0.1:11434",
    [switch]$ForceModelEval = $true,
    [switch]$ReplaceExisting = $true
)

$ErrorActionPreference = "Stop"

$scriptRoot = $PSScriptRoot
$env:SPIRE2MIND_MODEL_PROVIDER = "api"
$env:SPIRE2MIND_API_PROVIDER = "openai"
$env:SPIRE2MIND_API_BASE_URL = $BaseUrl
$env:SPIRE2MIND_API_KEY = ""
$env:SPIRE2MIND_MODEL = "qwen3:4b"
$env:SPIRE2MIND_API_DECISION_MODE = "structured"
$env:SPIRE2MIND_LOCAL_FALLBACK_MODEL = "qwen3:4b"
$env:SPIRE2MIND_FORCE_MODEL_EVAL = if ($ForceModelEval) { "1" } else { "0" }

$apiStartScript = Join-Path $scriptRoot "start-spire2mind-api.ps1"
$forceModelEvalValue = if ($ForceModelEval) { "1" } else { "0" }
$replaceExistingValue = if ($ReplaceExisting) { "1" } else { "0" }
& powershell.exe -ExecutionPolicy Bypass -File $apiStartScript `
    -Mode $Mode `
    -Attempts $Attempts `
    -MaxCycles $MaxCycles `
    -TimeoutSeconds $TimeoutSeconds `
    -IdleTimeoutSeconds $IdleTimeoutSeconds `
    -Language $Language `
    -FastMode $FastMode `
    -Planner $Planner `
    -ApiBaseUrl $BaseUrl `
    -ApiProvider "openai" `
    -ApiDecisionMode "structured" `
    -Model "qwen3:4b" `
    -ForceModelEval $forceModelEvalValue `
    -ReplaceExisting $replaceExistingValue

exit $LASTEXITCODE
