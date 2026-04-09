param()

$ErrorActionPreference = "Stop"

try {
    $processes = Get-CimInstance Win32_Process | Where-Object {
        $_.CommandLine -and $_.CommandLine -match 'melotts_server\.py'
    }
}
catch {
    $processes = @()
}

foreach ($process in $processes) {
    try {
        Stop-Process -Id $process.ProcessId -Force -ErrorAction Stop
        Write-Host "Stopped local MeloTTS server PID $($process.ProcessId)"
    }
    catch {
    }
}
