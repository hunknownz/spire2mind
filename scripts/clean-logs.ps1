[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [string]$ScratchRoot = "",
    [switch]$Force
)

$ErrorActionPreference = "Stop"

function Get-RepoRoot {
    return Split-Path -Parent $PSScriptRoot
}

function Resolve-ScratchRoot {
    param([string]$RequestedRoot)

    if ([string]::IsNullOrWhiteSpace($RequestedRoot)) {
        return Join-Path (Get-RepoRoot) "scratch"
    }

    return (Resolve-Path $RequestedRoot).Path
}

function Test-IsDefaultScratchRoot {
    param([string]$ResolvedScratchRoot)

    $defaultRoot = Join-Path (Get-RepoRoot) "scratch"
    return [string]::Equals(
        [System.IO.Path]::GetFullPath($ResolvedScratchRoot),
        [System.IO.Path]::GetFullPath($defaultRoot),
        [System.StringComparison]::OrdinalIgnoreCase
    )
}

function Get-LiveAgentProcesses {
    return @(Get-Process spire2mind -ErrorAction SilentlyContinue)
}

function Remove-Target {
    param(
        [string]$Path,
        [switch]$ChildrenOnly
    )

    if (-not (Test-Path $Path)) {
        return [pscustomobject]@{
            Path    = $Path
            Removed = $false
            Reason  = "missing"
        }
    }

    if ($ChildrenOnly) {
        $children = @(Get-ChildItem -Force $Path -ErrorAction SilentlyContinue)
        if ($children.Count -eq 0) {
            return [pscustomobject]@{
                Path    = $Path
                Removed = $false
                Reason  = "empty"
            }
        }

        if ($PSCmdlet.ShouldProcess($Path, "Remove log contents")) {
            foreach ($child in $children) {
                Remove-Item -LiteralPath $child.FullName -Recurse -Force
            }
        }

        return [pscustomobject]@{
            Path    = $Path
            Removed = $true
            Reason  = "cleared"
        }
    }

    if ($PSCmdlet.ShouldProcess($Path, "Remove log file")) {
        Remove-Item -LiteralPath $Path -Force
    }

    return [pscustomobject]@{
        Path    = $Path
        Removed = $true
        Reason  = "deleted"
    }
}

$resolvedScratchRoot = Resolve-ScratchRoot -RequestedRoot $ScratchRoot
$isDefaultRoot = Test-IsDefaultScratchRoot -ResolvedScratchRoot $resolvedScratchRoot

if (-not (Test-Path $resolvedScratchRoot)) {
    throw "Scratch root does not exist: $resolvedScratchRoot"
}

if ($isDefaultRoot -and -not $Force) {
    $live = Get-LiveAgentProcesses
    if ($live.Count -gt 0) {
        $pids = $live | Select-Object -ExpandProperty Id | Sort-Object
        throw "Active Spire2Mind processes detected: $($pids -join ', '). Stop them first or rerun with -Force."
    }
}

$targets = @(
    @{ Path = Join-Path $resolvedScratchRoot "agent-runs"; ChildrenOnly = $true },
    @{ Path = Join-Path $resolvedScratchRoot "manual-runs"; ChildrenOnly = $true },
    @{ Path = Join-Path $resolvedScratchRoot "local-llm-launch.log"; ChildrenOnly = $false },
    @{ Path = Join-Path $resolvedScratchRoot "state-live.json"; ChildrenOnly = $false },
    @{ Path = Join-Path $resolvedScratchRoot "state-live2.json"; ChildrenOnly = $false }
)

$results = foreach ($target in $targets) {
    Remove-Target -Path $target.Path -ChildrenOnly:$target.ChildrenOnly
}

Write-Host "Cleaned disposable Spire2Mind logs under $resolvedScratchRoot"
Write-Host ""
foreach ($result in $results) {
    Write-Host ("- {0}: {1}" -f $result.Path, $result.Reason)
}

Write-Host ""
Write-Host "Preserved:"
Write-Host ("- {0}" -f (Join-Path $resolvedScratchRoot "guidebook"))
Write-Host ("- {0}" -f (Join-Path $resolvedScratchRoot "wdac"))
