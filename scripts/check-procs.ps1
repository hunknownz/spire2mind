$procs = Get-CimInstance Win32_Process | Where-Object { $_.CommandLine }
$agent = $procs | Where-Object { $_.CommandLine -match 'spire2mind' }
$tts = $procs | Where-Object { $_.CommandLine -match 'tts-player' }
$melo = $procs | Where-Object { $_.CommandLine -match 'melotts_server' }
$tui = $procs | Where-Object { $_.CommandLine -match 'run-tui' }
Write-Host "=== Agent processes ==="
$agent | ForEach-Object { Write-Host "  PID=$($_.ProcessId) CMD=$($_.CommandLine.Substring(0, [Math]::Min(120, $_.CommandLine.Length)))" }
Write-Host "=== TTS player ==="
$tts | ForEach-Object { Write-Host "  PID=$($_.ProcessId)" }
Write-Host "=== MeloTTS server ==="
$melo | ForEach-Object { Write-Host "  PID=$($_.ProcessId)" }
Write-Host "=== TUI ==="
$tui | ForEach-Object { Write-Host "  PID=$($_.ProcessId)" }
