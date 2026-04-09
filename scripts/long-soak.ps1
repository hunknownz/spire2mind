param(
    [int]$Attempts = 3,
    [int]$TimeoutSeconds = 900,
    [int]$IdleTimeoutSeconds = 120,
    [string]$FastMode = "instant",
    [string]$Planner = "mcts"
)

$ErrorActionPreference = "Stop"

$scriptRoot = $PSScriptRoot
$started = Get-Date
$status = "ok"
$message = ""
$attemptLabel = if ($Attempts -le 0) { "continuous" } else { [string]$Attempts }

Write-Host ""
Write-Host "=== Long soak: $attemptLabel chained attempts ==="

try {
    & (Join-Path $scriptRoot "headless-smoke.ps1") `
        -TimeoutSeconds $TimeoutSeconds `
        -IdleTimeoutSeconds $IdleTimeoutSeconds `
        -Attempts $Attempts `
        -FastMode $FastMode `
        -Planner $Planner
}
catch {
    $status = "failed"
    $message = $_.Exception.Message
}

$finished = Get-Date
$result = [pscustomobject]@{
    attempts = $attemptLabel
    status = $status
    started = $started
    finished = $finished
    duration_seconds = [math]::Round(($finished - $started).TotalSeconds, 1)
    message = $message
}

$result | Format-List

if ($status -ne "ok") {
    throw "Long soak failed."
}
