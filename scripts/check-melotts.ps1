$count = (Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -match 'melotts_server' }).Count
Write-Host "melotts_server processes: $count"
$count2 = (Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -match 'kokoro_server' }).Count
Write-Host "kokoro_server processes: $count2"
$count3 = (Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -match 'tts-player' }).Count
Write-Host "tts-player processes: $count3"
