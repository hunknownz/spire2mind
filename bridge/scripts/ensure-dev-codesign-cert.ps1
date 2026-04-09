param(
    [string]$Subject = "CN=Spire2Mind Dev Code Signing",
    [int]$ValidYears = 5
)

$ErrorActionPreference = "Stop"

function Get-ExistingCertificate {
    param([string]$CertificateSubject)

    Get-ChildItem Cert:\CurrentUser\My -CodeSigningCert |
        Where-Object {
            $_.Subject -eq $CertificateSubject -and
            $_.HasPrivateKey -and
            $_.NotAfter -gt (Get-Date).AddDays(7)
        } |
        Sort-Object NotAfter -Descending |
        Select-Object -First 1
}

function Ensure-CertificateInStore {
    param(
        [System.Security.Cryptography.X509Certificates.X509Certificate2]$Certificate,
        [string]$StorePath
    )

    $existing = Get-ChildItem -Path $StorePath |
        Where-Object { $_.Thumbprint -eq $Certificate.Thumbprint } |
        Select-Object -First 1
    if ($existing) {
        return
    }

    $tempFile = Join-Path $env:TEMP ("spire2mind-codesign-" + [guid]::NewGuid().ToString("N") + ".cer")
    try {
        Export-Certificate -Cert $Certificate -FilePath $tempFile -Force | Out-Null
        Import-Certificate -FilePath $tempFile -CertStoreLocation $StorePath | Out-Null
    }
    finally {
        Remove-Item -LiteralPath $tempFile -Force -ErrorAction SilentlyContinue
    }
}

$certificate = Get-ExistingCertificate -CertificateSubject $Subject
if (-not $certificate) {
    $certificate = New-SelfSignedCertificate `
        -Subject $Subject `
        -Type CodeSigningCert `
        -CertStoreLocation Cert:\CurrentUser\My `
        -KeyExportPolicy Exportable `
        -KeyAlgorithm RSA `
        -KeyLength 3072 `
        -HashAlgorithm SHA256 `
        -NotAfter (Get-Date).AddYears($ValidYears)
}

Ensure-CertificateInStore -Certificate $certificate -StorePath Cert:\CurrentUser\Root
Ensure-CertificateInStore -Certificate $certificate -StorePath Cert:\CurrentUser\TrustedPublisher
Ensure-CertificateInStore -Certificate $certificate -StorePath Cert:\CurrentUser\TrustedPeople

[pscustomobject]@{
    subject = $certificate.Subject
    thumbprint = $certificate.Thumbprint
    not_after = $certificate.NotAfter.ToString("o")
} | ConvertTo-Json -Compress
