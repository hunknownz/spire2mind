# Spire2Mind Runbook

## Local prerequisites

- Steam install of Slay the Spire 2 at `v0.99.1`
- `.NET 9 SDK`
- Go toolchain, or the repo-local Go binary under `.tools/`
- local Bridge signing certificate
- local supplemental App Control policy on machines that block unsigned DLLs

## Build and install

Bridge:

```powershell
pwsh .\bridge\scripts\build.ps1
pwsh .\bridge\scripts\install.ps1
```

If Windows App Control blocks the DLL on a new machine:

```powershell
pwsh .\bridge\scripts\ensure-dev-codesign-cert.ps1
pwsh .\bridge\scripts\install-dev-appcontrol-policy.ps1
```

## Start the game

Launch:

`C:\Program Files (x86)\Steam\steamapps\common\Slay the Spire 2\SlayTheSpire2.exe`

Wait until `/health` reports `ready=true`.

## Doctor

Run:

```powershell
pwsh .\scripts\doctor.ps1
```

This checks:

- Bridge health
- state, markdown state, and action endpoints
- SSE availability
- game data directory
- artifacts directory
- game executable path
- model configuration and model probe

Model backends:

- `SPIRE2MIND_MODEL_PROVIDER=claude-cli`
  Uses the local logged-in Claude Code CLI through our forked
  `github.com/hunknownz/open-agent-sdk-go`.
- `SPIRE2MIND_MODEL_PROVIDER=api`
  Uses `SPIRE2MIND_API_BASE_URL`, `SPIRE2MIND_API_KEY`, and `SPIRE2MIND_MODEL`
  or compatible `ANTHROPIC_*` variables.
  For local Ollama/OpenAI-compatible services, also set
  `SPIRE2MIND_API_PROVIDER=openai`.
  Loopback URLs such as `http://127.0.0.1:11434` do not require an API key.

### Local LLM with Ollama

Preferred local launcher:

```powershell
pwsh .\scripts\start-spire2mind-local-llm.ps1 -Mode tui
```

This launcher:

- ensures Ollama is installed and reachable on `http://127.0.0.1:11434`
- points Spire2Mind at the OpenAI-compatible Ollama API
- defaults to `qwen3:8b` as the primary model
- supports `qwen3:4b` as a smaller fallback model you can pre-pull with `-PullModel`

Examples:

```powershell
pwsh .\scripts\start-spire2mind-local-llm.ps1 -Mode tui -PullModel
pwsh .\scripts\start-spire2mind-local-llm.ps1 -Mode headless-smoke -Attempts 2
pwsh .\scripts\start-spire2mind-local-llm.ps1 -Mode long-soak -Attempts 3
```

## Headless smoke

Run:

```powershell
pwsh .\scripts\headless-smoke.ps1 -TimeoutSeconds 300
```

Outputs go to `scratch/manual-runs/`.
To chain multiple attempts inside a single runtime session:

```powershell
pwsh .\scripts\headless-smoke.ps1 -TimeoutSeconds 900 -Attempts 2
```

## Long soak

Run:

```powershell
pwsh .\scripts\long-soak.ps1 -Attempts 3 -TimeoutSeconds 900
```

This now runs one unattended headless session that chains the requested number of attempts and stops on the first failure.

## Main artifact locations

- live Bridge log:
  `C:\Users\klerc\AppData\Roaming\SlayTheSpire2\logs\godot.log`
- unattended run artifacts:
  `scratch\agent-runs\`
- manual smoke outputs:
  `scratch\manual-runs\`

## Watching the live run

If the Bubble Tea TUI is not directly visible in your terminal session, you can watch the persisted dashboard mirror instead:

```powershell
pwsh .\scripts\watch-dashboard.ps1 -ShowStory
```

This follows the latest directory under `scratch\agent-runs\` and refreshes the current dashboard plus the evolving run story.
The dashboard now mirrors more of the runtime cockpit:

- current goals and room intent
- carry-forward lessons from previous defeats
- room detail for combat, rewards, map, shop, rest, chest, and game over
- the latest reflection story once a run ends

For a cross-run view, inspect `scratch\guidebook\`:

- `guidebook.md` for merged strategy, recovery hotspots, and recent attempts
- `living-codex.json` for the structured world/discovery snapshot across recent runs

## Current validation target

The current project target is:

- Windows Steam Slay the Spire 2
- version `v0.99.1`
- local Bridge + Go runtime
- single-player only
