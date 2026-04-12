$scriptRoot = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $MyInvocation.MyCommand.Path }
$repoRoot = Split-Path -Parent $scriptRoot
Write-Host "scriptRoot: $scriptRoot"
Write-Host "repoRoot: $repoRoot"
$utilsPath = Join-Path $scriptRoot "tts-profile-utils.ps1"
Write-Host "utils path: $utilsPath"
Write-Host "utils exists: $(Test-Path $utilsPath)"
. $utilsPath
$p = Resolve-TTSProfile -RepoRoot $repoRoot -ProfileName "melotts-default"
Write-Host "profile type: $($p.GetType().Name)"
Write-Host "profile name: $($p.name)"
Write-Host "profile provider: $($p.provider)"
