param(
    [switch]$VerboseOutput
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$preferredGo = Join-Path $repoRoot ".tools\go-go1.26.1 go1.25.8\go\bin\go.exe"
$goExe = if (Test-Path $preferredGo) { $preferredGo } else { "go" }
$cacheRoot = Join-Path $repoRoot ".cache"
$goCache = Join-Path $cacheRoot "gocache"
$goModCache = Join-Path $cacheRoot "gomodcache"

New-Item -ItemType Directory -Force -Path $goCache | Out-Null
New-Item -ItemType Directory -Force -Path $goModCache | Out-Null

$env:GOCACHE = $goCache
$env:GOMODCACHE = $goModCache

$suites = @(
    @{
        Name = "agent action contracts"
        Package = "./internal/agent"
        Pattern = @(
            'TestActionInvariant',
            'TestExecuteModelDecisionWaitsThroughActionWindowChangeBeforeActing',
            'TestExecuteDirectActionStabilizesSameScreenIndexDriftBeforeNormalizing',
            'TestExecuteDirectActionRejectsTargetedPlayCardWithoutTargetBeforeBridgeCall',
            'TestExecuteDirectActionFreshensEndTurnStateBeforeActing',
            'TestChooseDeterministicActionAdvancesFinishedEvent',
            'TestChooseRuleBasedActionAdvancesFinishedEventSingleAction'
        ) -join '|'
    },
    @{
        Name = "reward/selection/game-over state contracts"
        Package = "./internal/game"
        Pattern = @(
            'TestStateInvariantReward',
            'TestStateInvariantSelection',
            'TestStateInvariantGameOver',
            'TestStateInvariantFinishedEvent'
        ) -join '|'
    },
    @{
        Name = "combat action-window invariants"
        Package = "./internal/game"
        Pattern = @(
            'TestStateInvariantCombat',
            'TestWaitUntilActionable'
        ) -join '|'
    }
)

foreach ($suite in $suites) {
    $args = @('test', $suite.Package, '-run', $suite.Pattern)
    if ($VerboseOutput) {
        $args += '-v'
    }

    Write-Host ("Running {0}..." -f $suite.Name) -ForegroundColor Cyan
    & $goExe @args
    if ($LASTEXITCODE -ne 0) {
        throw ("Invariant suite failed: {0}" -f $suite.Name)
    }
}

Write-Host "Invariant test suite passed." -ForegroundColor Green
