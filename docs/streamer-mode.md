# Streamer Mode

## Goal

Turn the agent into a live game streamer instead of a silent controller.

The streamer layer does not own gameplay.
It observes the current game state and the recent local run context, then
produces audience-facing commentary and TTS text.

## Pipeline

1. Game state arrives in the runtime loop.
2. The harness records the state, local goals, and recent important events.
3. On important beats, the streamer layer builds a separate commentary prompt:
   - current screen and room detail
   - local run context
   - recent event history
   - current trigger such as map choice, reward, shop, combat opening, or game over
4. The model returns a structured payload:
   - mood
   - commentary
   - game insight
   - life reflection
   - tts_text
5. A separate speech-splitting stage turns `tts_text` into `tts_segments` for spoken rhythm.
6. The harness:
   - emits a `streamer` event
   - writes `scratch/tts/latest.json`
   - writes `scratch/tts/latest.txt`
   - appends a timestamped payload under `scratch/tts/queue/`
7. A separate TTS sidecar can read the queue, handle interrupts/replacement, and speak it locally.

## Current Trigger Policy

The first implementation only comments on important beats:

- map choice
- reward choice
- card selection
- event choice
- shop choice
- rest choice
- chest
- game over
- combat opening

This keeps the streamer layer readable and avoids spamming every action.

## Runtime Boundaries

- Gameplay decision-making still uses:
  - deterministic
  - planner/MCTS
  - structured model decision
- The streamer layer is separate and non-blocking for correctness.
- If commentary generation fails, gameplay continues.

## TTS

The runtime now writes a queue under:

- `scratch/tts/`

Legacy fallback watcher:

- `scripts/watch-tts-queue.ps1`

reads `latest.json` and speaks `tts_text` with Windows speech synthesis.

Preferred runtime sidecar:

- `scripts/start-tts-player.ps1`
- `tools/tts-player/index.mjs`

The sidecar follows AIRI-style boundaries:

- the game runtime writes structured beats
- the speech splitter produces `tts_segments` for spoken pacing
- the sidecar owns queueing, interruption, and playback
- synthesis provider can be swapped independently

### Local TTS provider strategy

For local deployment, keep the transport and the engine separate:

- transport:
  - `openai-compatible /audio/speech`
- engine behind that transport:
  - `kokoro`
  - `melotts`
- fallback:
  - `windows-sapi`

This matches the AIRI-style direction:

- use a uniform `/audio/speech` interface where possible
- keep queueing / interruption / playback inside the local sidecar
- let the actual voice engine be replaceable

Current sidecar provider values:

- `windows-sapi`
- `openai-compatible`
- `kokoro`
- `melotts`

### One-click switching

One-click voice switching is done outside the TUI.
It belongs to the local runtime profile layer so the choice survives restarts
and can safely restart the local TTS sidecar.

Use:

- `scripts/switch-tts-profile.ps1`

Built-in profiles:

- `melotts-default`
- `melotts-bright`
- `kokoro-cute`
- `kokoro-calm`

Examples:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\switch-tts-profile.ps1 -Profile melotts-bright
powershell -ExecutionPolicy Bypass -File .\scripts\switch-tts-profile.ps1 -Profile kokoro-cute
```

The switch script:

- writes `scratch/tts/provider-profile.json`
- restarts the local `tts-player` sidecar
- lets future game launches inherit the saved profile

Suggested local setup:

1. run a local or LAN TTS service that exposes `/audio/speech`
2. set:
   - `SPIRE2MIND_TTS_AUTO_SPEAK=1`
   - `SPIRE2MIND_TTS_PROVIDER=melotts` or `kokoro`
   - `SPIRE2MIND_TTS_BASE_URL=http://...`
   - `SPIRE2MIND_TTS_MODEL=...` if the backend requires an explicit model name
   - `SPIRE2MIND_TTS_VOICE=...` if the backend exposes multiple voices
3. keep `windows-sapi` as fallback when the network synthesis path fails

For Chinese female-streamer output, `melotts` is the preferred first local engine.
`kokoro` remains a good second option when a lighter local stack is preferred.

### Current local defaults

The runtime now defaults to a local MeloTTS setup:

- provider: `melotts`
- base URL: `http://127.0.0.1:18080`
- model: `melotts`
- voice: `female`
- fallback: `windows-sapi`

Recommended local backup provider:

- provider: `kokoro`
- base URL: `http://127.0.0.1:18081`
- model: `kokoro`
- voice: `zf_xiaoxiao`

## Next Iterations

- add richer combat triggers for big swings, lethal risk, and elite fights
- add separate broadcaster voice/style profiles
- let TUI show a rolling commentary timeline instead of only the latest beat
- optionally split gameplay model and streamer model
