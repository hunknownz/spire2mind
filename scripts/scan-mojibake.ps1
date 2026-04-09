param(
    [string]$RunDir = "",
    [int]$LatestRuns = 1,
    [switch]$IncludeGuidebook,
    [switch]$Watch,
    [int]$RefreshSeconds = 10,
    [switch]$FailOnHit
)

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

$repoRoot = Split-Path -Parent $PSScriptRoot
$agentRunsRoot = Join-Path $repoRoot "scratch\agent-runs"
$guidebookPath = Join-Path $repoRoot "scratch\guidebook\guidebook.md"

# Store mojibake signatures as Base64 so the script file itself stays ASCII-clean.
$patternBase64 = @(
    "6Za+5L265pWz",
    "6ZCj5bKE5r2w",
    "6Y2U44Sk57aU",
    "6Y695qi/7pum",
    "54Ge5YKb5pqf",
    "6ZCi54a35oeh",
    "6Zay5oid56u1",
    "6Y2l54Ky5oKO",
    "6ZCc4pWB7oaN",
    "6ZGz5LuL5Zm6",
    "6Y+N5YW85bCF",
    "6Y+E54a75YWY",
    "6Y+B5bG85rGJ",
    "6Y+H5p2R7pi/",
    "6Y615pKz5Zqu",
    "6ZeD5o+S5bC9",
    "6ZOU7oa85ZCU",
    "6Y2Z6Iy25bmI"
)

$patterns = @(
    $patternBase64 | ForEach-Object {
        [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($_))
    }
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

function Get-ScanTargets {
    param(
        [string[]]$RunDirectories,
        [switch]$WithGuidebook
    )

    $targets = New-Object System.Collections.Generic.List[string]
    foreach ($dir in $RunDirectories) {
        foreach ($name in @("dashboard.md", "events.jsonl")) {
            $path = Join-Path $dir $name
            if (Test-Path $path) {
                $targets.Add($path)
            }
        }

        Get-ChildItem -Path $dir -Filter "cycle-*.prompt.txt" -ErrorAction SilentlyContinue |
            Sort-Object Name |
            ForEach-Object { $targets.Add($_.FullName) }
    }

    if ($WithGuidebook -and (Test-Path $guidebookPath)) {
        $targets.Add($guidebookPath)
    }

    return @($targets)
}

function Invoke-MojibakeScan {
    param(
        [string[]]$Targets,
        [string[]]$Needles
    )

    $hits = New-Object System.Collections.Generic.List[object]
    foreach ($target in $Targets) {
        foreach ($needle in $Needles) {
            $matches = Select-String -Path $target -Pattern $needle -SimpleMatch -Encoding utf8 -ErrorAction SilentlyContinue
            foreach ($match in $matches) {
                $hits.Add([pscustomobject]@{
                    Path    = $match.Path
                    Line    = $match.LineNumber
                    Pattern = $needle
                    Text    = $match.Line.Trim()
                })
            }
        }
    }

    return @($hits)
}

function Write-ScanReport {
    param(
        [string[]]$RunDirectories,
        [object[]]$Hits,
        [switch]$WithGuidebook
    )

    Write-Host "Spire2Mind mojibake scan"
    Write-Host ("Runs: {0}" -f ($RunDirectories -join ", "))
    if ($WithGuidebook) {
        Write-Host ("Guidebook: {0}" -f $guidebookPath)
    }
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
    $targets = Get-ScanTargets -RunDirectories $runs -WithGuidebook:$IncludeGuidebook
    $hits = Invoke-MojibakeScan -Targets $targets -Needles $patterns

    if ($Watch) {
        Clear-Host
    }

    Write-ScanReport -RunDirectories $runs -Hits $hits -WithGuidebook:$IncludeGuidebook

    if ($FailOnHit -and $hits.Count -gt 0) {
        exit 1
    }

    if ($Watch) {
        Start-Sleep -Seconds ([Math]::Max(1, $RefreshSeconds))
    }
} while ($Watch)
