param(
    [ValidateSet("melotts-default", "melotts-bright", "kokoro-cute", "kokoro-calm")]
    [string]$Profile = "melotts-bright",
    [switch]$RestartPlayer = $true,
    [switch]$ShowProfiles
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot "tts-profile-utils.ps1")

if ($ShowProfiles) {
    $profiles = Get-TTSBuiltInProfiles
    foreach ($name in ($profiles.Keys | Sort-Object)) {
        $item = $profiles[$name]
        Write-Host "$name"
        Write-Host "  provider: $($item.provider)"
        Write-Host "  baseUrl:  $($item.baseUrl)"
        Write-Host "  voice:    $($item.voice)"
        Write-Host "  speed:    $($item.speed)"
        Write-Host "  style:    $($item.streamerStyle)"
    }
    exit 0
}

$resolved = Resolve-TTSProfile -RepoRoot $repoRoot -ProfileName $Profile
$profilePath = Save-TTSProfile -RepoRoot $repoRoot -Profile $resolved

Write-Host "Saved TTS profile: $Profile"
Write-Host "Path: $profilePath"
Write-Host "Provider: $($resolved.provider)"
Write-Host "Base URL: $($resolved.baseUrl)"
Write-Host "Voice: $($resolved.voice)"
Write-Host "Speed: $($resolved.speed)"
Write-Host "Streamer style: $($resolved.streamerStyle)"

if ($RestartPlayer) {
    & powershell.exe -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "start-tts-player.ps1") -ReplaceExisting
}
