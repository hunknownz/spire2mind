param(
    [switch]$SkipPck
)

$ErrorActionPreference = "Stop"

function Resolve-Dotnet {
    if (Test-Path "C:\Program Files\dotnet\dotnet.exe") {
        return "C:\Program Files\dotnet\dotnet.exe"
    }

    $cmd = Get-Command dotnet -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Source
    }

    throw "dotnet.exe not found. Install .NET 9 SDK first."
}

function Resolve-GodotConsole {
    $default = "C:\Users\$env:USERNAME\AppData\Local\Microsoft\WinGet\Packages\GodotEngine.GodotEngine.Mono_Microsoft.Winget.Source_8wekyb3d8bbwe\Godot_v4.5.1-stable_mono_win64\Godot_v4.5.1-stable_mono_win64_console.exe"
    if (Test-Path $default) {
        return $default
    }

    $candidate = Get-ChildItem "$env:LOCALAPPDATA\Microsoft\WinGet\Packages" -Recurse -Filter "*godot*_console.exe" -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($candidate) {
        return $candidate.FullName
    }

    $cmd = Get-Command godot_console -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Source
    }

    throw "godot_console executable not found. Install Godot 4.5.1 Mono first."
}

function Resolve-GameDir {
    if ($env:STS2_GAME_DIR) {
        return $env:STS2_GAME_DIR
    }

    $default = "C:\Program Files (x86)\Steam\steamapps\common\Slay the Spire 2"
    if (Test-Path $default) {
        return $default
    }

    throw "Could not resolve STS2_GAME_DIR."
}

function Ensure-GodotUserLogs {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ProjectName
    )

    $appDataRoot = Join-Path $env:APPDATA "Godot\app_userdata"
    $logsDir = Join-Path (Join-Path $appDataRoot $ProjectName) "logs"
    New-Item -ItemType Directory -Force $logsDir | Out-Null
}

$dotnet = Resolve-Dotnet
$godotConsole = Resolve-GodotConsole
$bridgeDir = Split-Path $PSScriptRoot -Parent
$gameDir = Resolve-GameDir
$dataDir = Join-Path $gameDir "data_sts2_windows_x86_64"
$buildDir = Join-Path $bridgeDir "build"
$projectDir = Join-Path $bridgeDir "pck"
$pckSourceDir = Join-Path $projectDir "src"
$pckOutput = Join-Path $buildDir "Spire2Mind.Bridge.pck"
$dllOutput = Join-Path $bridgeDir "bin\Release\net9.0\Spire2Mind.Bridge.dll"
$ensureCertScript = Join-Path $PSScriptRoot "ensure-dev-codesign-cert.ps1"
$signDllScript = Join-Path $PSScriptRoot "sign-bridge-dll.ps1"

New-Item -ItemType Directory -Force $buildDir | Out-Null
Ensure-GodotUserLogs -ProjectName "Spire2Mind Bridge Pack"

& $dotnet build (Join-Path $bridgeDir "Spire2Mind.Bridge.csproj") -c Release -nodeReuse:false -p:Sts2DataDir="$dataDir"
if ($LASTEXITCODE -ne 0) {
    throw "dotnet build failed."
}

if (-not (Test-Path $dllOutput)) {
    throw "Built DLL not found: $dllOutput"
}

& $ensureCertScript | Out-Host
if ($LASTEXITCODE -ne 0) {
    throw "Code-signing certificate setup failed."
}

& $signDllScript -Path $dllOutput | Out-Host
if ($LASTEXITCODE -ne 0) {
    throw "DLL signing failed."
}

if (-not $SkipPck) {
    & $godotConsole --headless --path $projectDir --script "res://scripts/pack_pck.gd" -- $pckSourceDir $pckOutput
    if ($LASTEXITCODE -ne 0) {
        throw "PCK build failed."
    }
}
elseif (-not (Test-Path $pckOutput)) {
    throw "SkipPck was requested but no existing PCK was found at $pckOutput"
}
else {
    Write-Host "Skipping PCK build and reusing existing PCK: $pckOutput"
}

Write-Host "Bridge DLL built and PCK packed."
Write-Host "DLL: $dllOutput"
Write-Host "PCK: $pckOutput"
