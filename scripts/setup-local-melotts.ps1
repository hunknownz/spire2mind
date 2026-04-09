param(
    [string]$PythonExe = "",
    [string]$VenvPath = "",
    [switch]$UpgradePip = $true
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$meloRoot = Join-Path $repoRoot "research\MeloTTS"
if (-not (Test-Path $meloRoot)) {
    throw "MeloTTS repo not found at $meloRoot"
}

if ([string]::IsNullOrWhiteSpace($PythonExe)) {
    $candidate = Join-Path $env:LOCALAPPDATA "Programs\Python\Python311\python.exe"
    if (-not (Test-Path $candidate)) {
        throw "Python 3.11 not found. Install Python first or pass -PythonExe."
    }
    $PythonExe = $candidate
}

if ([string]::IsNullOrWhiteSpace($VenvPath)) {
    $VenvPath = Join-Path $repoRoot ".tools\melotts\venv"
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

& $venvPython -m pip install fastapi uvicorn
& $venvPython -m pip install -e $meloRoot
& $venvPython -m unidic download

Write-Host "MeloTTS local environment is ready."
Write-Host "Python: $venvPython"
