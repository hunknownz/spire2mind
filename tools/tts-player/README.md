# Spire2Mind TTS Player

Local sidecar that watches `scratch/tts/latest.json`, applies simple queue and interrupt rules, and plays speech locally.

Current providers:

- `windows-sapi`
- `openai-compatible`
- `melotts`
- `kokoro`

Designed after AIRI's audio pipeline boundaries:

- text generation remains in the main app
- the sidecar owns queueing, interruption, and playback
- synthesis provider can be swapped without changing game logic

Recommended local setup:

- default: `melotts`
- backup: `kokoro`
- fallback: `windows-sapi`
