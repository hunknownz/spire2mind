$ErrorActionPreference = "Stop"

try {
    $processes = Get-CimInstance Win32_Process | Where-Object {
        $_.CommandLine -and $_.CommandLine -match 'tools\\tts-player\\index\.mjs'
    }
}
catch {
    $processes = @()
}

foreach ($process in $processes) {
    try {
        Stop-Process -Id $process.ProcessId -Force -ErrorAction Stop
        Write-Host "Stopped TTS player: $($process.ProcessId)"
    }
    catch {
        Write-Warning "Failed to stop TTS player $($process.ProcessId): $($_.Exception.Message)"
    }
}
