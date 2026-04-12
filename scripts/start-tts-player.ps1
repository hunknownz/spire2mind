param(
    [string]$TtsRoot = "",
    [ValidateSet("windows-sapi", "openai-compatible", "kokoro", "melotts")]
    [string]$Provider = "",
    [ValidateSet("windows-sapi")]
    [string]$FallbackProvider = "",
    [string]$BaseUrl = "",
    [string]$ApiKey = "",
    [string]$Model = "",
    [string]$Voice = "",
    [string]$Speed = "",
    [string]$Profile = "",
    [string]$ReplaceExisting = "1",
    [switch]$Visible,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot "tts-profile-utils.ps1")
if ([string]::IsNullOrWhiteSpace($TtsRoot)) {
    $TtsRoot = Join-Path $repoRoot "scratch\tts"
}

$resolvedProfile = Resolve-TTSProfile -RepoRoot $repoRoot -ProfileName $Profile
$resolvedProvider = if ([string]::IsNullOrWhiteSpace($Provider)) { [string]$resolvedProfile.provider } else { $Provider }
$resolvedFallbackProvider = if ([string]::IsNullOrWhiteSpace($FallbackProvider)) { [string]$resolvedProfile.fallbackProvider } else { $FallbackProvider }
$resolvedBaseUrl = if ([string]::IsNullOrWhiteSpace($BaseUrl)) { [string]$resolvedProfile.baseUrl } else { $BaseUrl }
$resolvedModel = if ([string]::IsNullOrWhiteSpace($Model)) { [string]$resolvedProfile.model } else { $Model }
$resolvedVoice = if ([string]::IsNullOrWhiteSpace($Voice)) { [string]$resolvedProfile.voice } else { $Voice }
$resolvedSpeed = if ([string]::IsNullOrWhiteSpace($Speed)) { [string]$resolvedProfile.speed } else { $Speed }

function Resolve-NodeExe {
    $node = Get-ChildItem -Path (Join-Path $repoRoot ".tools\node") -Directory -Filter "node-*-win-x64" -ErrorAction SilentlyContinue |
        Sort-Object Name -Descending |
        Select-Object -First 1
    if (-not $node) {
        throw "Portable Node runtime not found under .tools\\node"
    }

    $nodeExe = Join-Path $node.FullName "node.exe"
    if (-not (Test-Path $nodeExe)) {
        throw "node.exe not found at $nodeExe"
    }
    return $nodeExe
}

function Stop-ExistingTTSPlayer {
    param(
        [int]$CurrentPid
    )

    try {
        $processes = Get-CimInstance Win32_Process | Where-Object {
            $_.CommandLine -and $_.CommandLine -match 'tools\\tts-player\\index\.mjs'
        }
    }
    catch {
        return
    }

    foreach ($process in $processes) {
        if ($process.ProcessId -eq $CurrentPid) {
            continue
        }
        try {
            Stop-Process -Id $process.ProcessId -Force -ErrorAction Stop
        }
        catch {
        }
    }
}

$nodeExe = Resolve-NodeExe
$entry = Join-Path $repoRoot "tools\tts-player\index.mjs"
if (-not (Test-Path $entry)) {
    throw "TTS sidecar entry not found: $entry"
}

if ($ReplaceExisting -match '^(1|true|yes|on)$') {
    Stop-ExistingTTSPlayer -CurrentPid $PID
}

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $nodeExe
$psi.WorkingDirectory = $repoRoot
$psi.UseShellExecute = $false
$psi.CreateNoWindow = -not $Visible.IsPresent
$arguments = @('"' + $entry + '"')
if ($DryRun) {
    $arguments += "--dry-run"
}
$psi.Arguments = [string]::Join(" ", $arguments)
$psi.Environment["SPIRE2MIND_REPO_ROOT"] = $repoRoot
$psi.Environment["SPIRE2MIND_TTS_ROOT"] = $TtsRoot
$psi.Environment["SPIRE2MIND_TTS_PROVIDER"] = $resolvedProvider
$psi.Environment["SPIRE2MIND_TTS_FALLBACK_PROVIDER"] = $resolvedFallbackProvider
$psi.Environment["SPIRE2MIND_TTS_BASE_URL"] = $resolvedBaseUrl
$psi.Environment["SPIRE2MIND_TTS_API_KEY"] = $ApiKey
$psi.Environment["SPIRE2MIND_TTS_MODEL"] = $resolvedModel
$psi.Environment["SPIRE2MIND_TTS_VOICE"] = $resolvedVoice
$psi.Environment["SPIRE2MIND_TTS_SPEED"] = $resolvedSpeed

$process = [System.Diagnostics.Process]::Start($psi)
if (-not $process) {
    throw "Failed to start TTS player."
}

Write-Host "Started TTS player: PID $($process.Id)"
Write-Host "Profile: $([string]$resolvedProfile.name)"
Write-Host "Provider: $resolvedProvider"
Write-Host "Fallback provider: $resolvedFallbackProvider"
if (-not [string]::IsNullOrWhiteSpace($resolvedBaseUrl)) {
    Write-Host "Base URL: $resolvedBaseUrl"
}
if (-not [string]::IsNullOrWhiteSpace($resolvedModel)) {
    Write-Host "Model: $resolvedModel"
}
if (-not [string]::IsNullOrWhiteSpace($resolvedVoice)) {
    Write-Host "Voice: $resolvedVoice"
}
if (-not [string]::IsNullOrWhiteSpace($resolvedSpeed)) {
    Write-Host "Speed: $resolvedSpeed"
}
