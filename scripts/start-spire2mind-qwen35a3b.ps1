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
    [string]$BaseUrl = "http://192.168.3.23:11434",
    [string]$FallbackModel = "qwen3:8b",
    [int]$ModelContext = 32768,
    [switch]$ForceModelEval = $true,
    [switch]$ReplaceExisting = $true
)

$ErrorActionPreference = "Stop"

$scriptRoot = $PSScriptRoot
$env:SPIRE2MIND_MODEL_PROVIDER = "api"
$env:SPIRE2MIND_API_PROVIDER = "openai"
$env:SPIRE2MIND_API_BASE_URL = $BaseUrl
$env:SPIRE2MIND_API_KEY = ""
$env:SPIRE2MIND_API_DECISION_MODE = "structured"
$env:SPIRE2MIND_MODEL = "qwen3.5:35b-a3b"
$env:SPIRE2MIND_MODEL_CONTEXT = [string]$ModelContext
$env:SPIRE2MIND_LOCAL_FALLBACK_MODEL = $FallbackModel
$env:SPIRE2MIND_FORCE_MODEL_EVAL = if ($ForceModelEval) { "1" } else { "0" }

$localStartScript = Join-Path $scriptRoot "start-spire2mind-local-llm.ps1"
$forceModelEvalValue = if ($ForceModelEval) { "1" } else { "0" }
$replaceExistingValue = if ($ReplaceExisting) { "1" } else { "0" }
& powershell.exe -ExecutionPolicy Bypass -File $localStartScript `
    -Mode $Mode `
    -Attempts $Attempts `
    -MaxCycles $MaxCycles `
    -TimeoutSeconds $TimeoutSeconds `
    -IdleTimeoutSeconds $IdleTimeoutSeconds `
    -Language $Language `
    -FastMode $FastMode `
    -Planner $Planner `
    -BaseUrl $BaseUrl `
    -Model "qwen3.5:35b-a3b" `
    -FallbackModel $FallbackModel `
    -ModelContext $ModelContext `
    -ApiDecisionMode "structured" `
    -ForceModelEval $forceModelEvalValue `
    -ReplaceExisting $replaceExistingValue

exit $LASTEXITCODE
