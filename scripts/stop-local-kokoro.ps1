param()

$ErrorActionPreference = "Stop"

try {
    $processes = Get-CimInstance Win32_Process | Where-Object {
        $_.CommandLine -and $_.CommandLine -match 'kokoro_server\.py'
    }
}
catch {
    $processes = @()
}

foreach ($process in $processes) {
    try {
        Stop-Process -Id $process.ProcessId -Force -ErrorAction Stop
        Write-Host "Stopped local Kokoro server PID $($process.ProcessId)"
    }
    catch {
    }
}
