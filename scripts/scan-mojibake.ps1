param(
    [string]$RunDir = "",
    [int]$LatestRuns = 1,
    [switch]$IncludeGuidebook,
    [switch]$IncludeTTS = $true,
    [switch]$IncludeManualRuns = $true,
    [switch]$SkipManualRuns,
    [switch]$Watch,
    [int]$RefreshSeconds = 10,
    [switch]$FailOnHit
)

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

$repoRoot = Split-Path -Parent $PSScriptRoot
$agentRunsRoot = Join-Path $repoRoot "scratch\agent-runs"
$manualRunsRoot = Join-Path $repoRoot "scratch\manual-runs"
$guidebookPath = Join-Path $repoRoot "scratch\guidebook\guidebook.md"
$ttsRoot = Join-Path $repoRoot "scratch\tts"

function New-MojibakePattern {
    param([int[]]$CodePoints)

    return (-join ($CodePoints | ForEach-Object { [char]$_ }))
}

$patterns = @(
    (New-MojibakePattern -CodePoints @(0x5BEE, 0x20AC)),
    (New-MojibakePattern -CodePoints @(0x8924, 0x6483)),
    (New-MojibakePattern -CodePoints @(0x9354, 0x310E, 0x7DB5)),
    (New-MojibakePattern -CodePoints @(0x7459, 0x89E3, 0xE87A)),
    (New-MojibakePattern -CodePoints @(0x93C8, 0x20AC, 0x8FD1)),
    (New-MojibakePattern -CodePoints @(0x95C3, 0x9632, 0x6307)),
    (New-MojibakePattern -CodePoints @(0x951F)),
    (New-MojibakePattern -CodePoints @(0x9227))
)

function Resolve-RunDirectories {
    param(
        [string]$RequestedRunDir,
        [int]$Count
    )

    if (-not [string]::IsNullOrWhiteSpace($RequestedRunDir)) {
        if (-not (Test-Path $RequestedRunDir)) {
            throw "Run directory does not exist: $RequestedRunDir"
        }
        return @((Resolve-Path $RequestedRunDir).Path)
    }

    $dirs = Get-ChildItem -Path $agentRunsRoot -Directory -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First ([Math]::Max(1, $Count))

    if ($null -eq $dirs -or $dirs.Count -eq 0) {
        throw "No agent run directories found under $agentRunsRoot"
    }

    return @($dirs | ForEach-Object { $_.FullName })
}

function Add-IfExists {
    param(
        [System.Collections.Generic.List[string]]$Targets,
        [string]$Path
    )

    if (Test-Path $Path) {
        $Targets.Add((Resolve-Path $Path).Path)
    }
}

function Get-ScanTargets {
    param(
        [string[]]$RunDirectories,
        [switch]$WithGuidebook,
        [switch]$WithTTS,
        [switch]$WithManualRuns
    )

    $targets = [System.Collections.Generic.List[string]]::new()

    foreach ($dir in $RunDirectories) {
        foreach ($name in @("dashboard.md", "events.jsonl")) {
            Add-IfExists -Targets $targets -Path (Join-Path $dir $name)
        }

        Get-ChildItem -Path $dir -Filter "cycle-*.prompt.txt" -ErrorAction SilentlyContinue |
            Sort-Object Name |
            ForEach-Object { $targets.Add($_.FullName) }
    }

    if ($WithGuidebook) {
        Add-IfExists -Targets $targets -Path $guidebookPath
    }

    if ($WithTTS -and (Test-Path $ttsRoot)) {
        foreach ($name in @("latest.json", "latest.txt", "player.log")) {
            Add-IfExists -Targets $targets -Path (Join-Path $ttsRoot $name)
        }

        $queueDir = Join-Path $ttsRoot "queue"
        if (Test-Path $queueDir) {
            Get-ChildItem -Path $queueDir -Filter "*.json" -ErrorAction SilentlyContinue |
                Sort-Object LastWriteTime -Descending |
                Select-Object -First 20 |
                ForEach-Object { $targets.Add($_.FullName) }
        }
    }

    if ($WithManualRuns -and (Test-Path $manualRunsRoot)) {
        Get-ChildItem -Path $manualRunsRoot -Directory -ErrorAction SilentlyContinue |
            Sort-Object LastWriteTime -Descending |
            Select-Object -First 3 |
            ForEach-Object {
                foreach ($name in @("stdout.log", "stderr.log")) {
                    Add-IfExists -Targets $targets -Path (Join-Path $_.FullName $name)
                }
            }
    }

    return @($targets | Select-Object -Unique)
}

function Invoke-MojibakeScan {
    param(
        [string[]]$Targets,
        [string[]]$Needles
    )

    $hits = @()
    foreach ($target in $Targets) {
        foreach ($needle in $Needles) {
            $matches = Select-String -Path $target -Pattern $needle -SimpleMatch -Encoding utf8 -ErrorAction SilentlyContinue
            foreach ($match in $matches) {
                $hits += [pscustomobject]@{
                    Path    = $match.Path
                    Line    = $match.LineNumber
                    Pattern = $needle
                    Text    = $match.Line.Trim()
                }
            }
        }
    }

    return @($hits)
}

function Write-ScanReport {
    param(
        [string[]]$RunDirectories,
        [string[]]$Targets,
        [object[]]$Hits,
        [switch]$WithGuidebook,
        [switch]$WithTTS,
        [switch]$WithManualRuns
    )

    Write-Host "Spire2Mind mojibake scan"
    Write-Host ("Runs: {0}" -f ($RunDirectories -join ", "))
    if ($WithGuidebook) {
        Write-Host ("Guidebook: {0}" -f $guidebookPath)
    }
    if ($WithTTS) {
        Write-Host ("TTS root: {0}" -f $ttsRoot)
    }
    if ($WithManualRuns) {
        Write-Host ("Manual runs: {0}" -f $manualRunsRoot)
    }
    Write-Host ("Files: {0}" -f $Targets.Count)
    Write-Host ("Checked at: {0}" -f (Get-Date -Format "yyyy-MM-dd HH:mm:ss"))
    Write-Host ""

    if ($Hits.Count -eq 0) {
        Write-Host "No mojibake patterns found." -ForegroundColor Green
        return
    }

    Write-Host ("Found {0} mojibake hit(s)." -f $Hits.Count) -ForegroundColor Yellow
    foreach ($hit in $Hits) {
        Write-Host ("[{0}] {1}:{2}" -f $hit.Pattern, $hit.Path, $hit.Line) -ForegroundColor Red
        Write-Host ("  {0}" -f $hit.Text)
    }
}

do {
    $runs = Resolve-RunDirectories -RequestedRunDir $RunDir -Count $LatestRuns
    $withManualRuns = $IncludeManualRuns -and (-not $SkipManualRuns)
    $targets = Get-ScanTargets -RunDirectories $runs -WithGuidebook:$IncludeGuidebook -WithTTS:$IncludeTTS -WithManualRuns:$withManualRuns
    $hits = Invoke-MojibakeScan -Targets $targets -Needles $patterns

    if ($Watch) {
        Clear-Host
    }

    Write-ScanReport -RunDirectories $runs -Targets $targets -Hits $hits -WithGuidebook:$IncludeGuidebook -WithTTS:$IncludeTTS -WithManualRuns:$withManualRuns

    if ($FailOnHit -and $hits.Count -gt 0) {
        Write-Error ("Found {0} mojibake hit(s)." -f $hits.Count)
        [System.Environment]::Exit(1)
    }

    if ($Watch) {
        Start-Sleep -Seconds ([Math]::Max(1, $RefreshSeconds))
    }
} while ($Watch)
