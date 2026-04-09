param(
    [string]$PythonExe = "",
    [string]$VenvPath = "",
    [switch]$InstallESpeak = $true,
    [switch]$UpgradePip = $true
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot

if ([string]::IsNullOrWhiteSpace($PythonExe)) {
    $candidate = Join-Path $env:LOCALAPPDATA "Programs\Python\Python311\python.exe"
    if (-not (Test-Path $candidate)) {
        throw "Python 3.11 not found. Install Python first or pass -PythonExe."
    }
    $PythonExe = $candidate
}

if ([string]::IsNullOrWhiteSpace($VenvPath)) {
    $VenvPath = Join-Path $repoRoot ".tools\kokoro\venv"
}

if ($InstallESpeak) {
    $hasEspeak = Get-Command espeak-ng -ErrorAction SilentlyContinue
    if (-not $hasEspeak) {
        winget install -e --id eSpeak-NG.eSpeak-NG --accept-package-agreements --accept-source-agreements
    }
}

if (-not (Test-Path (Join-Path $VenvPath "Scripts\python.exe"))) {
    & $PythonExe -m venv $VenvPath
}

$venvPython = Join-Path $VenvPath "Scripts\python.exe"
if (-not (Test-Path $venvPython)) {
    throw "Virtual environment python not found at $venvPython"
}

if ($UpgradePip) {
    & $venvPython -m pip install --upgrade pip setuptools wheel
}

& $venvPython -m pip install fastapi uvicorn soundfile "kokoro>=0.9.4" "misaki[zh]>=0.9.4"

Write-Host "Kokoro local environment is ready."
Write-Host "Python: $venvPython"
