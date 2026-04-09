function Get-TTSProfilePath {
    param(
        [string]$RepoRoot
    )

    return Join-Path $RepoRoot "scratch\tts\provider-profile.json"
}

function Get-TTSBuiltInProfiles {
    return @{
        "melotts-default" = @{
            name = "melotts-default"
            label = "MeloTTS Default"
            provider = "melotts"
            fallbackProvider = "windows-sapi"
            baseUrl = "http://127.0.0.1:18080"
            model = "melotts"
            voice = "female"
            speed = 1.00
            streamerStyle = "warm"
        }
        "melotts-bright" = @{
            name = "melotts-bright"
            label = "MeloTTS Bright"
            provider = "melotts"
            fallbackProvider = "windows-sapi"
            baseUrl = "http://127.0.0.1:18080"
            model = "melotts"
            voice = "female"
            speed = 1.10
            streamerStyle = "bright-cute"
        }
        "kokoro-cute" = @{
            name = "kokoro-cute"
            label = "Kokoro Cute"
            provider = "kokoro"
            fallbackProvider = "windows-sapi"
            baseUrl = "http://127.0.0.1:18081"
            model = "kokoro"
            voice = "zf_xiaoxiao"
            speed = 1.08
            streamerStyle = "bright-cute"
        }
        "kokoro-calm" = @{
            name = "kokoro-calm"
            label = "Kokoro Calm"
            provider = "kokoro"
            fallbackProvider = "windows-sapi"
            baseUrl = "http://127.0.0.1:18081"
            model = "kokoro"
            voice = "zf_xiaoxiao"
            speed = 0.96
            streamerStyle = "calm"
        }
    }
}

function ConvertTo-TTSProfileHashtable {
    param(
        [object]$InputObject
    )

    if ($null -eq $InputObject) {
        return $null
    }

    $result = @{}
    foreach ($property in $InputObject.PSObject.Properties) {
        $result[$property.Name] = $property.Value
    }
    return $result
}

function Get-TTSSavedProfile {
    param(
        [string]$RepoRoot
    )

    $profilePath = Get-TTSProfilePath -RepoRoot $RepoRoot
    if (-not (Test-Path $profilePath)) {
        return $null
    }

    $raw = Get-Content -Path $profilePath -Raw -Encoding UTF8
    if ([string]::IsNullOrWhiteSpace($raw)) {
        return $null
    }

    return ConvertTo-TTSProfileHashtable -InputObject ($raw | ConvertFrom-Json)
}

function Resolve-TTSProfile {
    param(
        [string]$RepoRoot,
        [string]$ProfileName = ""
    )

    $profiles = Get-TTSBuiltInProfiles
    $selectedName = ""
    if (-not [string]::IsNullOrWhiteSpace($ProfileName)) {
        $selectedName = $ProfileName.Trim().ToLowerInvariant()
    }
    if (-not [string]::IsNullOrWhiteSpace($selectedName)) {
        if (-not $profiles.ContainsKey($selectedName)) {
            throw "Unknown TTS profile: $ProfileName"
        }
        return $profiles[$selectedName].Clone()
    }

    $saved = Get-TTSSavedProfile -RepoRoot $RepoRoot
    if ($null -ne $saved) {
        return $saved
    }

    return $profiles["melotts-default"].Clone()
}

function Save-TTSProfile {
    param(
        [string]$RepoRoot,
        [hashtable]$Profile
    )

    $profilePath = Get-TTSProfilePath -RepoRoot $RepoRoot
    $profileDir = Split-Path -Parent $profilePath
    New-Item -ItemType Directory -Force -Path $profileDir | Out-Null

    $json = $Profile | ConvertTo-Json -Depth 4
    Set-Content -Path $profilePath -Value $json -Encoding UTF8
    return $profilePath
}
