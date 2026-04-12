param(
    [int]$Port = 18081,
    [string]$BindAddress = "127.0.0.1",
    [string]$VenvPath = "",
    [string]$LanguageCode = "z",
    [string]$Voice = "zf_xiaoxiao",
    [double]$Speed = 1.0,
    [switch]$Visible,
    [string]$ReplaceExisting = "1"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$serverPath = Join-Path $repoRoot "tools\local-tts\kokoro_server.py"
if (-not (Test-Path $serverPath)) {
    throw "Kokoro server script not found: $serverPath"
}

if ([string]::IsNullOrWhiteSpace($VenvPath)) {
    $VenvPath = Join-Path $repoRoot ".tools\kokoro\venv"
}
$venvPython = Join-Path $VenvPath "Scripts\python.exe"
if (-not (Test-Path $venvPython)) {
    throw "Kokoro venv not found. Run scripts\\setup-local-kokoro.ps1 first."
}

function Stop-ExistingKokoroServer {
    param([int]$CurrentPid)

    try {
        $processes = Get-CimInstance Win32_Process | Where-Object {
            $_.CommandLine -and $_.CommandLine -match 'kokoro_server\.py'
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
    Stop-ExistingKokoroServer -CurrentPid $PID
}

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $venvPython
$psi.WorkingDirectory = $repoRoot
$psi.UseShellExecute = $false
$psi.CreateNoWindow = -not $Visible.IsPresent
$psi.Arguments = "-m uvicorn kokoro_server:app --app-dir `"$($serverPath | Split-Path -Parent)`" --host $BindAddress --port $Port"
$psi.Environment["SPIRE2MIND_KOKORO_LANGUAGE_CODE"] = $LanguageCode
$psi.Environment["SPIRE2MIND_KOKORO_VOICE"] = $Voice
$psi.Environment["SPIRE2MIND_KOKORO_SPEED"] = [string]$Speed

$process = [System.Diagnostics.Process]::Start($psi)
if (-not $process) {
    throw "Failed to start local Kokoro server."
}

Write-Host "Started local Kokoro server: PID $($process.Id)"
Write-Host "URL: http://$BindAddress`:$Port/audio/speech"
Write-Host "Language code: $LanguageCode"
Write-Host "Voice: $Voice"
