param(
    [ValidateSet("speak", "wav")]
    [string]$Mode = "speak",
    [string]$Text = "",
    [string]$VoiceName = "",
    [int]$Rate = 0,
    [string]$WavPath = ""
)

$ErrorActionPreference = "Stop"

switch ($Mode) {
    "speak" {
        Add-Type -AssemblyName System.Speech
        $synth = New-Object System.Speech.Synthesis.SpeechSynthesizer
        $synth.Rate = [Math]::Max(-10, [Math]::Min(10, $Rate))

        if (-not [string]::IsNullOrWhiteSpace($VoiceName)) {
            try {
                $synth.SelectVoice($VoiceName)
            }
            catch {
            }
        } else {
            try {
                $female = $synth.GetInstalledVoices() |
                    ForEach-Object { $_.VoiceInfo } |
                    Where-Object { $_.Gender -eq [System.Speech.Synthesis.VoiceGender]::Female } |
                    Select-Object -First 1
                if ($female) {
                    $synth.SelectVoice($female.Name)
                }
            }
            catch {
            }
        }

        if (-not [string]::IsNullOrWhiteSpace($Text)) {
            $synth.Speak($Text)
        }
    }
    "wav" {
        if ([string]::IsNullOrWhiteSpace($WavPath) -or -not (Test-Path -LiteralPath $WavPath)) {
            throw "WAV file not found: $WavPath"
        }
        Add-Type -AssemblyName System
        $player = New-Object System.Media.SoundPlayer
        $player.SoundLocation = $WavPath
        $player.Load()
        $player.PlaySync()
    }
}
