param(
    [string]$TtsRoot = "",
    [int]$PollSeconds = 2,
    [string]$VoiceName = ""
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($TtsRoot)) {
    $repoRoot = Split-Path -Parent $PSScriptRoot
    $TtsRoot = Join-Path $repoRoot "scratch\tts"
}

$latestPath = Join-Path $TtsRoot "latest.json"
$seenStamp = ""

Add-Type -AssemblyName System.Speech
$synth = New-Object System.Speech.Synthesis.SpeechSynthesizer

if (-not [string]::IsNullOrWhiteSpace($VoiceName)) {
    try {
        $synth.SelectVoice($VoiceName)
    }
    catch {
        Write-Warning "Voice not found: $VoiceName"
    }
}

Write-Host "Watching TTS queue: $TtsRoot"

while ($true) {
    if (Test-Path $latestPath) {
        try {
            $json = Get-Content -LiteralPath $latestPath -Raw -Encoding UTF8 | ConvertFrom-Json
            $stamp = ""
            if ($json.PSObject.Properties.Name -contains "trigger") {
                $stamp += [string]$json.trigger
            }
            if ($json.PSObject.Properties.Name -contains "tts_text") {
                $stamp += "|" + [string]$json.tts_text
            }
            if ($stamp -ne "" -and $stamp -ne $seenStamp) {
                $seenStamp = $stamp
                $text = [string]$json.tts_text
                if (-not [string]::IsNullOrWhiteSpace($text)) {
                    Write-Host ""
                    Write-Host "[TTS] $text"
                    $synth.SpeakAsyncCancelAll()
                    $null = $synth.SpeakAsync($text)
                }
            }
        }
        catch {
            Write-Warning "Failed to read TTS payload: $($_.Exception.Message)"
        }
    }

    Start-Sleep -Seconds ([Math]::Max(1, $PollSeconds))
}
