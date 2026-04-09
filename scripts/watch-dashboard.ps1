param(
    [string]$RunDir = "",
    [int]$RefreshSeconds = 2,
    [switch]$ShowStory
)

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

$repoRoot = Split-Path -Parent $PSScriptRoot
$agentRunsRoot = Join-Path $repoRoot "scratch\agent-runs"

function Resolve-RunDirectory {
    param([string]$RequestedRunDir)

    if (-not [string]::IsNullOrWhiteSpace($RequestedRunDir)) {
        if (-not (Test-Path $RequestedRunDir)) {
            throw "Run directory does not exist: $RequestedRunDir"
        }
        return (Resolve-Path $RequestedRunDir).Path
    }

    $latest = Get-ChildItem -Path $agentRunsRoot -Directory -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1

    if ($null -eq $latest) {
        throw "No agent run directories found under $agentRunsRoot"
    }

    return $latest.FullName
}

function Read-FileOrPlaceholder {
    param([string]$Path, [string]$Title)

    if (-not (Test-Path $Path)) {
        return @(
            "# $Title",
            "",
            "_Waiting for file: $Path_"
        ) -join "`r`n"
    }

    return Get-Content -Path $Path -Raw -Encoding utf8
}

while ($true) {
    $resolvedRunDir = Resolve-RunDirectory -RequestedRunDir $RunDir
    $dashboardPath = Join-Path $resolvedRunDir "dashboard.md"
    $storyPath = Join-Path $resolvedRunDir "run-story.md"

    $dashboard = Read-FileOrPlaceholder -Path $dashboardPath -Title "Dashboard"
    $story = if ($ShowStory) { Read-FileOrPlaceholder -Path $storyPath -Title "Run Story" } else { "" }

    Clear-Host
    Write-Host "Spire2Mind dashboard watcher"
    Write-Host "Run dir: $resolvedRunDir"
    Write-Host "Refresh: ${RefreshSeconds}s"
    Write-Host ""
    Write-Host $dashboard

    if ($ShowStory) {
        Write-Host ""
        Write-Host ("-" * 80)
        Write-Host ""
        Write-Host $story
    }

    Start-Sleep -Seconds $RefreshSeconds
}
