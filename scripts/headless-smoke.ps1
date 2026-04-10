param(
    [int]$TimeoutSeconds = 300,
    [int]$IdleTimeoutSeconds = 90,
    [int]$Attempts = 1,
    [string]$FastMode = "instant",
    [string]$Planner = "mcts"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$preferredGo = Join-Path $repoRoot ".tools\go-go1.26.1 go1.25.8\go\bin\go.exe"
$goExe = if (Test-Path $preferredGo) { $preferredGo } else { "go" }
$bridgeUrl = if ($env:SPIRE2MIND_BRIDGE_URL) { $env:SPIRE2MIND_BRIDGE_URL.TrimEnd("/") } else { "http://127.0.0.1:8080" }
$gameDir = if ($env:STS2_GAME_DIR) { $env:STS2_GAME_DIR } else { "C:\Program Files (x86)\Steam\steamapps\common\Slay the Spire 2" }
$gameExe = Join-Path $gameDir "SlayTheSpire2.exe"

$outputRoot = Join-Path $repoRoot "scratch\manual-runs"
$null = New-Item -ItemType Directory -Force -Path $outputRoot
$runId = "{0}-{1}" -f (Get-Date -Format "yyyyMMdd-HHmmss-headless-smoke"), ([guid]::NewGuid().ToString("N").Substring(0, 8))
$runDir = Join-Path $outputRoot $runId
$null = New-Item -ItemType Directory -Force -Path $runDir

$stdoutPath = Join-Path $runDir "stdout.log"
$stderrPath = Join-Path $runDir "stderr.log"
$artifactsRoot = Join-Path $repoRoot "scratch\agent-runs"
$guidebookRoot = Join-Path $repoRoot "scratch\guidebook"

$cacheRoot = Join-Path $repoRoot ".cache"
$null = New-Item -ItemType Directory -Force -Path $cacheRoot
$goCache = Join-Path $cacheRoot "gocache"
$goModCache = Join-Path $cacheRoot "gomodcache"
$null = New-Item -ItemType Directory -Force -Path $goCache
$null = New-Item -ItemType Directory -Force -Path $goModCache

$processEnvironment = [ordered]@{}
$seenEnvNames = New-Object 'System.Collections.Generic.HashSet[string]' ([System.StringComparer]::OrdinalIgnoreCase)
foreach ($entry in [System.Environment]::GetEnvironmentVariables("Process").GetEnumerator()) {
    $name = [string]$entry.Key
    if (-not $seenEnvNames.Add($name)) {
        continue
    }
    $processEnvironment[$name] = [string]$entry.Value
}

$resolvedPath = $null
if ($processEnvironment.Contains("Path")) {
    $resolvedPath = $processEnvironment["Path"]
}
elseif ($processEnvironment.Contains("PATH")) {
    $resolvedPath = $processEnvironment["PATH"]
}
if ($null -ne $resolvedPath) {
    $processEnvironment["Path"] = $resolvedPath
    $processEnvironment.Remove("PATH") | Out-Null
}

$processEnvironment["GOCACHE"] = $goCache
$processEnvironment["GOMODCACHE"] = $goModCache
$processEnvironment["GOTELEMETRY"] = "off"
$processEnvironment["GOPROXY"] = if ($processEnvironment.Contains("GOPROXY") -and [string]::IsNullOrWhiteSpace($processEnvironment["GOPROXY"]) -eq $false) {
    $processEnvironment["GOPROXY"]
} else {
    "https://proxy.golang.org,direct"
}
$processEnvironment["SPIRE2MIND_GAME_FAST_MODE"] = $FastMode
$processEnvironment["SPIRE2MIND_COMBAT_PLANNER"] = $Planner

function Set-ConsoleUtf8 {
    try {
        & cmd /c chcp 65001 > $null
    }
    catch {
    }

    try {
        $utf8 = [System.Text.UTF8Encoding]::new($false)
        [Console]::InputEncoding = $utf8
        [Console]::OutputEncoding = $utf8
        $global:OutputEncoding = $utf8
    }
    catch {
    }
}

function Test-BridgeReady {
    param(
        [string]$Url
    )

    try {
        $response = Invoke-RestMethod -Uri "$Url/health" -TimeoutSec 3
        if ($null -eq $response) {
            return $false
        }
        if ($response.ready -eq $true) {
            return $true
        }
        if ($response.ok -eq $true -and $null -ne $response.data -and $response.data.ready -eq $true) {
            return $true
        }
        return $false
    }
    catch {
        return $false
    }
}

function Ensure-GameAndBridgeReady {
    param(
        [string]$ExecutablePath,
        [string]$Url,
        [int]$TimeoutSec
    )

    if (-not (Get-Process -Name "SlayTheSpire2" -ErrorAction SilentlyContinue | Select-Object -First 1)) {
        if (-not (Test-Path $ExecutablePath)) {
            throw "Game executable not found at $ExecutablePath"
        }
        Start-Process -FilePath $ExecutablePath | Out-Null
    }

    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if (Test-BridgeReady -Url $Url) {
            return
        }
        Start-Sleep -Seconds 2
    }

    throw "Bridge did not become ready within $TimeoutSec seconds."
}

function Get-LatestFileWriteTicks {
    param(
        [string]$Path
    )

    if (-not (Test-Path $Path)) {
        return 0L
    }

    $latest = Get-ChildItem -Path $Path -Recurse -File -Force -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTimeUtc -Descending |
        Select-Object -First 1
    if ($null -eq $latest) {
        return 0L
    }

    return $latest.LastWriteTimeUtc.Ticks
}

Write-Host "Using Go executable: $goExe"
Write-Host "Output directory: $runDir"
Write-Host "Fast mode: $FastMode"
Write-Host "Planner: $Planner"
Write-Host "Bridge URL: $bridgeUrl"
Write-Host "Game executable: $gameExe"

Ensure-GameAndBridgeReady -ExecutablePath $gameExe -Url $bridgeUrl -TimeoutSec 120

$job = Start-Job -ScriptBlock {
    param($repoRoot, $goExe, $attempts, $stdoutPath, $stderrPath, $environmentOverrides)

    Set-Location $repoRoot
    try {
        & cmd /c chcp 65001 > $null
    }
    catch {
    }
    try {
        $utf8 = [System.Text.UTF8Encoding]::new($false)
        [Console]::InputEncoding = $utf8
        [Console]::OutputEncoding = $utf8
        $global:OutputEncoding = $utf8
    }
    catch {
    }
    foreach ($entry in $environmentOverrides.GetEnumerator()) {
        [System.Environment]::SetEnvironmentVariable([string]$entry.Key, [string]$entry.Value, "Process")
    }

    & $goExe run .\cmd\spire2mind play --headless --attempts $attempts 1>> $stdoutPath 2>> $stderrPath
    if ($LASTEXITCODE -ne 0) {
        throw "Headless smoke failed with exit code $LASTEXITCODE."
    }
} -ArgumentList $repoRoot, $goExe, $Attempts, $stdoutPath, $stderrPath, $processEnvironment

try {
    $started = Get-Date
    $lastProgress = $started
    $lastStdoutLength = 0L
    $lastStderrLength = 0L
    $lastArtifactsTicks = Get-LatestFileWriteTicks -Path $artifactsRoot
    $lastGuidebookTicks = Get-LatestFileWriteTicks -Path $guidebookRoot

    while ($job.State -eq "Running" -or $job.State -eq "NotStarted") {
        Start-Sleep -Seconds 5
        $job = Get-Job -Id $job.Id

        $stdoutLength = if (Test-Path $stdoutPath) { (Get-Item $stdoutPath).Length } else { 0L }
        $stderrLength = if (Test-Path $stderrPath) { (Get-Item $stderrPath).Length } else { 0L }
        $artifactsTicks = Get-LatestFileWriteTicks -Path $artifactsRoot
        $guidebookTicks = Get-LatestFileWriteTicks -Path $guidebookRoot
        if (
            $stdoutLength -ne $lastStdoutLength -or
            $stderrLength -ne $lastStderrLength -or
            $artifactsTicks -gt $lastArtifactsTicks -or
            $guidebookTicks -gt $lastGuidebookTicks
        ) {
            $lastProgress = Get-Date
            $lastStdoutLength = $stdoutLength
            $lastStderrLength = $stderrLength
            $lastArtifactsTicks = $artifactsTicks
            $lastGuidebookTicks = $guidebookTicks
        }

        $now = Get-Date
        if (($now - $started).TotalSeconds -gt $TimeoutSeconds) {
            Write-Warning "Headless smoke hit total timeout after $TimeoutSeconds seconds. Stopping job."
            Stop-Job -Id $job.Id | Out-Null
            throw "Headless smoke timed out."
        }

        if (($now - $lastProgress).TotalSeconds -gt $IdleTimeoutSeconds) {
            Write-Warning "Headless smoke made no observable progress for $IdleTimeoutSeconds seconds. Stopping job."
            Stop-Job -Id $job.Id | Out-Null
            throw "Headless smoke stalled."
        }
    }

    $job = Get-Job -Id $job.Id
    $jobOutput = Receive-Job -Id $job.Id -Keep
    if ($jobOutput) {
        $jobOutput | Out-Null
    }

    Write-Host "Stdout: $stdoutPath"
    Write-Host "Stderr: $stderrPath"

    if ($job.State -eq "Failed") {
        $reason = if ($job.ChildJobs.Count -gt 0 -and $job.ChildJobs[0].JobStateInfo.Reason) {
            $job.ChildJobs[0].JobStateInfo.Reason.Message
        } else {
            "Headless smoke job failed."
        }
        throw $reason
    }
}
finally {
    if ($job -and ($job.State -eq "Running" -or $job.State -eq "NotStarted")) {
        Stop-Job -Id $job.Id | Out-Null
    }
    if ($job) {
        Remove-Job -Id $job.Id -Force -ErrorAction SilentlyContinue
    }
}
