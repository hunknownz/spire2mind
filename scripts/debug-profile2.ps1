$scriptRoot = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $MyInvocation.MyCommand.Path }
$repoRoot = Split-Path -Parent $scriptRoot
. (Join-Path $scriptRoot "tts-profile-utils.ps1")

$raw = Resolve-TTSProfile -RepoRoot $repoRoot -ProfileName "melotts-default"
Write-Host "raw type: $($raw.GetType().FullName)"
Write-Host "raw count: $(if ($raw -is [array]) { $raw.Count } else { 'N/A' })"
if ($raw -is [array]) {
    for ($i = 0; $i -lt $raw.Count; $i++) {
        Write-Host "  [$i] type=$($raw[$i].GetType().Name) value=$($raw[$i])"
    }
}
Write-Host "provider: $($raw.provider)"
