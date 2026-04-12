$scriptRoot = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $MyInvocation.MyCommand.Path }
$repoRoot = Split-Path -Parent $scriptRoot
. (Join-Path $scriptRoot "tts-profile-utils.ps1")
$raw = Resolve-TTSProfile -RepoRoot $repoRoot -ProfileName "melotts-default"
Write-Host "Keys: $($raw.Keys -join ', ')"
Write-Host "name=$($raw['name']) provider=$($raw['provider'])"
