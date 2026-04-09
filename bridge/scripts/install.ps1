param(
    [switch]$SkipPck
)

$ErrorActionPreference = "Stop"

$bridgeDir = Split-Path $PSScriptRoot -Parent
$gameDir = if ($env:STS2_GAME_DIR) { $env:STS2_GAME_DIR } else { "C:\Program Files (x86)\Steam\steamapps\common\Slay the Spire 2" }
$modsDir = Join-Path $gameDir "mods"
$modDll = Join-Path $modsDir "Spire2Mind.Bridge.dll"
$runningGame = Get-Process -Name "SlayTheSpire2" -ErrorAction SilentlyContinue | Select-Object -First 1

if ($runningGame) {
    throw "SlayTheSpire2 is still running. Close the game before installing a new Bridge DLL."
}

& (Join-Path $PSScriptRoot "build.ps1") -SkipPck:$SkipPck
if ($LASTEXITCODE -ne 0) {
    throw "Build step failed."
}

New-Item -ItemType Directory -Force $modsDir | Out-Null

Copy-Item (Join-Path $bridgeDir "bin\Release\net9.0\Spire2Mind.Bridge.dll") $modDll -Force
Copy-Item (Join-Path $bridgeDir "build\Spire2Mind.Bridge.pck") (Join-Path $modsDir "Spire2Mind.Bridge.pck") -Force
Copy-Item (Join-Path $bridgeDir "mod_id.json") (Join-Path $modsDir "mod_id.json") -Force

& (Join-Path $PSScriptRoot "sign-bridge-dll.ps1") -Path $modDll | Out-Host
if ($LASTEXITCODE -ne 0) {
    throw "Installed DLL signing failed."
}

Write-Host "Installed Bridge MVP into $modsDir"
