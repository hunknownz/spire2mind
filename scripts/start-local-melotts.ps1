param(
    [int]$Port = 18080,
    [string]$BindAddress = "127.0.0.1",
    [string]$VenvPath = "",
    [ValidateSet("cpu", "auto", "cuda", "cuda:0")]
    [string]$Device = "cpu",
    [string]$Language = "ZH",
    [string]$Speaker = "ZH",
    [double]$Speed = 1.0,
    [switch]$Visible,
    [string]$ReplaceExisting = "1"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$serverPath = Join-Path $repoRoot "tools\local-tts\melotts_server.py"
if (-not (Test-Path $serverPath)) {
    throw "MeloTTS server script not found: $serverPath"
}

if ([string]::IsNullOrWhiteSpace($VenvPath)) {
    $VenvPath = Join-Path $repoRoot ".tools\melotts\venv"
}
$venvPython = Join-Path $VenvPath "Scripts\python.exe"
if (-not (Test-Path $venvPython)) {
    throw "MeloTTS venv not found. Run scripts\\setup-local-melotts.ps1 first."
}

function Stop-ExistingMeloTTSServer {
    param([int]$CurrentPid)

    try {
        $processes = Get-CimInstance Win32_Process | Where-Object {
            $_.CommandLine -and $_.CommandLine -match 'melotts_server\.py'
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

if ($ReplaceExisting -match '^(1|true|yes|on)$') {
    Stop-ExistingMeloTTSServer -CurrentPid $PID
}

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $venvPython
$psi.WorkingDirectory = $repoRoot
$psi.UseShellExecute = $false
$psi.CreateNoWindow = -not $Visible.IsPresent
$psi.Arguments = "-m uvicorn melotts_server:app --app-dir `"$($serverPath | Split-Path -Parent)`" --host $BindAddress --port $Port"
$psi.Environment["SPIRE2MIND_MELOTTS_DEVICE"] = $Device
$psi.Environment["SPIRE2MIND_MELOTTS_LANGUAGE"] = $Language
$psi.Environment["SPIRE2MIND_MELOTTS_SPEAKER"] = $Speaker
$psi.Environment["SPIRE2MIND_MELOTTS_SPEED"] = [string]$Speed

$process = [System.Diagnostics.Process]::Start($psi)
if (-not $process) {
    throw "Failed to start local MeloTTS server."
}

Write-Host "Started local MeloTTS server: PID $($process.Id)"
Write-Host "URL: http://$BindAddress`:$Port/audio/speech"
Write-Host "Device: $Device"
Write-Host "Language: $Language"
Write-Host "Speaker: $Speaker"
