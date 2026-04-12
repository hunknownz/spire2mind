[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [string]$BindHost = "0.0.0.0",
    [int]$Port = 8080,
    [string]$RuleName = "Spire2Mind Bridge LAN 8080",
    [string]$RemoteAddress = "LocalSubnet",
    [string]$GameDir = "",
    [int]$TimeoutSeconds = 120,
    [switch]$NoLaunchGame,
    [switch]$SkipFirewall
)

$ErrorActionPreference = "Stop"

function Test-Administrator {
    $current = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($current)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Invoke-SelfElevation {
    param(
        [string]$ScriptPath
    )

    $argList = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", $ScriptPath,
        "-BindHost", $BindHost,
        "-Port", [string]$Port,
        "-RuleName", $RuleName,
        "-RemoteAddress", $RemoteAddress,
        "-TimeoutSeconds", [string]$TimeoutSeconds
    )

    if (-not [string]::IsNullOrWhiteSpace($GameDir)) {
        $argList += @("-GameDir", $GameDir)
    }
    if ($NoLaunchGame) {
        $argList += "-NoLaunchGame"
    }
    if ($SkipFirewall) {
        $argList += "-SkipFirewall"
    }

    Start-Process -FilePath "powershell.exe" -Verb RunAs -ArgumentList $argList | Out-Null
}

function Test-BridgeReady {
    param(
        [string]$Url
    )

    try {
        $response = Invoke-RestMethod -Uri "$Url/health" -TimeoutSec 3
        if ($null -eq $response) {
            return $false
        }
        if ($response.ready -eq $true) {
            return $true
        }
        if ($response.ok -eq $true -and $null -ne $response.data -and $response.data.ready -eq $true) {
            return $true
        }
        return $false
    }
    catch {
        return $false
    }
}

function Ensure-BridgeLanFirewallRule {
    param(
        [string]$DisplayName,
        [int]$LocalPort,
        [string]$Address
    )

    $rules = @(Get-NetFirewallRule -DisplayName $DisplayName -ErrorAction SilentlyContinue)
    if ($rules.Count -eq 0) {
        New-NetFirewallRule `
            -DisplayName $DisplayName `
            -Direction Inbound `
            -Action Allow `
            -Enabled True `
            -Profile Any `
            -Protocol TCP `
            -LocalPort $LocalPort `
            -RemoteAddress $Address | Out-Null
        return "created"
    }

    foreach ($rule in $rules) {
        Set-NetFirewallRule -InputObject $rule -Enabled True -Direction Inbound -Action Allow -Profile Any | Out-Null
    }

    $rules | Get-NetFirewallPortFilter | Set-NetFirewallPortFilter -Protocol TCP -LocalPort $LocalPort | Out-Null
    $rules | Get-NetFirewallAddressFilter | Set-NetFirewallAddressFilter -RemoteAddress $Address | Out-Null
    return "updated"
}

function Get-LanEndpoints {
    $addresses = @(Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object {
        $_.IPAddress -and
        $_.IPAddress -notlike "127.*" -and
        $_.IPAddress -notlike "169.254.*" -and
        $_.InterfaceAlias -notmatch "Loopback|vEthernet|Virtual|Teredo" -and
        (
            $_.IPAddress -match '^10\.' -or
            $_.IPAddress -match '^192\.168\.' -or
            $_.IPAddress -match '^172\.(1[6-9]|2[0-9]|3[0-1])\.'
        )
    } | Sort-Object InterfaceIndex, IPAddress)

    if ($addresses.Count -eq 0) {
        return @()
    }

    return @($addresses | Select-Object -ExpandProperty IPAddress -Unique)
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$gameRoot = if ([string]::IsNullOrWhiteSpace($GameDir)) {
    if ($env:STS2_GAME_DIR) { $env:STS2_GAME_DIR } else { "C:\Program Files (x86)\Steam\steamapps\common\Slay the Spire 2" }
} else {
    $GameDir
}
$gameExe = Join-Path $gameRoot "SlayTheSpire2.exe"
$bridgeUrl = "http://127.0.0.1:$Port"

if (-not $SkipFirewall -and -not (Test-Administrator) -and -not $WhatIfPreference) {
    Invoke-SelfElevation -ScriptPath $PSCommandPath
    exit
}

Set-Location $repoRoot
$env:STS2_API_HOST = $BindHost
$env:STS2_API_BIND = $BindHost
$env:STS2_API_PORT = [string]$Port

Write-Host "Bridge bind host: $BindHost"
Write-Host "Bridge port: $Port"
Write-Host "Game executable: $gameExe"
Write-Host "Firewall rule: $RuleName"

if (-not $SkipFirewall) {
    if ($PSCmdlet.ShouldProcess("Windows Firewall", "Allow inbound TCP $Port from $RemoteAddress")) {
        $result = Ensure-BridgeLanFirewallRule -DisplayName $RuleName -LocalPort $Port -Address $RemoteAddress
        Write-Host "Firewall rule ${result}: $RuleName"
    }
}
else {
    Write-Host "Skipping firewall setup."
}

$gameProcess = Get-Process -Name "SlayTheSpire2" -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $NoLaunchGame) {
    if (-not $gameProcess) {
        if (-not (Test-Path $gameExe)) {
            throw "Game executable not found at $gameExe"
        }

        if ($PSCmdlet.ShouldProcess($gameExe, "Start Slay the Spire 2")) {
            Start-Process -FilePath $gameExe | Out-Null
            Write-Host "Started game."
        }
    }
    else {
        Write-Host "Game is already running."
        Write-Host "If this session was started before setting STS2_API_HOST=0.0.0.0, restart the game once for LAN binding to take effect."
    }
}
else {
    Write-Host "Game launch skipped."
}

if ($WhatIfPreference) {
    Write-Host "Dry run complete."
    return
}

if ($NoLaunchGame -and -not $gameProcess) {
    $lanIps = @(Get-LanEndpoints)
    if ($lanIps.Count -gt 0) {
        Write-Host "LAN endpoints:"
        foreach ($ip in $lanIps) {
            Write-Host "  http://${ip}:$Port"
        }
    }
    return
}

$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
while ((Get-Date) -lt $deadline) {
    if (Test-BridgeReady -Url $bridgeUrl) {
        break
    }
    Start-Sleep -Seconds 2
}

if (-not (Test-BridgeReady -Url $bridgeUrl)) {
    throw "Bridge did not become ready at $bridgeUrl within $TimeoutSeconds seconds."
}

$lanIps = @(Get-LanEndpoints)
Write-Host "Bridge health: $bridgeUrl/health"
if ($lanIps.Count -gt 0) {
    Write-Host "LAN endpoints:"
    foreach ($ip in $lanIps) {
        Write-Host "  http://${ip}:$Port"
    }
}
