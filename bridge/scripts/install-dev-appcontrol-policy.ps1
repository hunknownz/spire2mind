param(
    [string]$BinaryPolicyPath,
    [string]$BridgeDllPath,
    [guid]$BasePolicyId = [guid]"60fd87f8-4593-44a0-91b0-2e0da022f248",
    [guid]$PolicyId = [guid]"6f31f6b4-ec77-4d7d-bcd0-4668f3790c71",
    [string]$PolicyName = "Spire2Mind Dev Supplemental",
    [switch]$SkipRefresh
)

$ErrorActionPreference = "Stop"

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Resolve-CiToolPath {
    $path = Join-Path $env:WINDIR "System32\CiTool.exe"
    if (-not (Test-Path $path)) {
        throw "CiTool.exe not found at $path"
    }

    return $path
}

function Resolve-BinaryPolicyPath {
    param([string]$Candidate)

    if ($Candidate) {
        if (-not (Test-Path $Candidate)) {
            throw "Supplemental policy not found: $Candidate"
        }

        return (Resolve-Path $Candidate).Path
    }

    $generator = Join-Path $PSScriptRoot "new-dev-appcontrol-supplemental-policy.ps1"
    try {
        $payload = @(& $generator -BridgeDllPath $BridgeDllPath -BasePolicyId $BasePolicyId -PolicyId $PolicyId -PolicyName $PolicyName)
    }
    catch {
        throw "Failed to generate the supplemental policy. $($_.Exception.Message)"
    }

    $jsonPayload = $payload |
        ForEach-Object { "$_" } |
        Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
        Select-Object -Last 1

    if (-not $jsonPayload) {
        throw "Supplemental policy generation did not return JSON metadata."
    }

    $result = $jsonPayload | ConvertFrom-Json
    if (-not (Test-Path $result.binary_path)) {
        throw "Generated supplemental policy not found: $($result.binary_path)"
    }

    return $result.binary_path
}

if (-not (Test-IsAdministrator)) {
    throw "install-dev-appcontrol-policy.ps1 must run in an elevated PowerShell window."
}

$ciTool = Resolve-CiToolPath
$resolvedBinaryPolicyPath = Resolve-BinaryPolicyPath -Candidate $BinaryPolicyPath

& $ciTool --update-policy $resolvedBinaryPolicyPath -json | Out-Host
if ($LASTEXITCODE -ne 0) {
    throw "CiTool failed to add or update the supplemental policy."
}

if (-not $SkipRefresh) {
    & $ciTool --refresh -json | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "CiTool refresh failed."
    }
}

[pscustomobject]@{
    binary_path = $resolvedBinaryPolicyPath
    policy_id = $PolicyId.ToString()
    base_policy_id = $BasePolicyId.ToString()
    refreshed = (-not $SkipRefresh)
} | ConvertTo-Json -Compress
