$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$preferredGo = Join-Path $repoRoot ".tools\go-go1.26.1 go1.25.8\go\bin\go.exe"
$goExe = if (Test-Path $preferredGo) { $preferredGo } else { "go" }

Write-Host "Using Go executable: $goExe"
Push-Location $repoRoot
try {
    & $goExe run .\cmd\spire2mind doctor
    if ($LASTEXITCODE -ne 0) {
        throw "spire2mind doctor failed with exit code $LASTEXITCODE."
    }
}
finally {
    Pop-Location
}
