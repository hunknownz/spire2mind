$ErrorActionPreference = "Stop"

$baseUrl = if ($env:STS2_API_BASE) { $env:STS2_API_BASE } else { "http://127.0.0.1:8080" }

Write-Host "Checking $baseUrl/health"
Invoke-RestMethod "$baseUrl/health" | ConvertTo-Json -Depth 6

Write-Host ""
Write-Host "Checking $baseUrl/state"
Invoke-RestMethod "$baseUrl/state" | ConvertTo-Json -Depth 8

Write-Host ""
Write-Host "Checking $baseUrl/actions/available"
Invoke-RestMethod "$baseUrl/actions/available" | ConvertTo-Json -Depth 8
