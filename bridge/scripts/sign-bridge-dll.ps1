param(
    [Parameter(Mandatory = $true)]
    [string]$Path,
    [string]$Subject = "CN=Spire2Mind Dev Code Signing"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $Path)) {
    throw "File not found: $Path"
}

$certificate = Get-ChildItem Cert:\CurrentUser\My -CodeSigningCert |
    Where-Object { $_.Subject -eq $Subject -and $_.HasPrivateKey } |
    Sort-Object NotAfter -Descending |
    Select-Object -First 1

if (-not $certificate) {
    throw "Code-signing certificate not found for subject '$Subject'. Run ensure-dev-codesign-cert.ps1 first."
}

$signature = Set-AuthenticodeSignature -FilePath $Path -Certificate $certificate -HashAlgorithm SHA256 -IncludeChain All
if ($signature.Status -notin @("Valid", "UnknownError")) {
    throw "Signing failed for $Path. Status: $($signature.Status) Message: $($signature.StatusMessage)"
}

[pscustomobject]@{
    path = (Resolve-Path $Path).Path
    status = $signature.Status.ToString()
    thumbprint = $certificate.Thumbprint
} | ConvertTo-Json -Compress
