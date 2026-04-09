# Local LLM Integration

## Goal

This document captures the current local LLM integration work for Spire2Mind.

The immediate goal is to reduce or replace expensive Claude API-backed runtime
usage with a local model that can support:

- continuous game play
- unattended smoke and soak runs
- parallel agent development while the runtime is actively playing

The chosen direction is not a new model architecture.
It is a provider swap on top of the existing `api` runtime path.

## Current Status

As of `2026-04-08`, local model support is operational on the current machine.

Confirmed working:

- Ollama installed on Windows
- Ollama local API reachable at `http://127.0.0.1:11434`
- local OpenAI-compatible API path wired into Spire2Mind
- model probe via `spire2mind doctor`
- local provider launcher script
- provider auto-selection updated to prefer configured API over Claude CLI

Downloaded local models:

- `qwen3:8b`
- `qwen3:4b`

Recommended usage on the current machine:

- primary model: `qwen3:8b`
- fallback / lighter model: `qwen3:4b`

## Machine Constraints

The current development machine was checked directly.

Hardware snapshot:

- GPU: `NVIDIA GeForce RTX 3060 12GB`
- system memory: about `16GB`

Implications:

- practical long-run local model range is `4B` to `8B` quantized models
- `8B` is the largest reasonable default tier for continuous use here
- `14B+` models are not a good default for unattended play on this machine
- long-run reliability is more important than maximizing raw benchmark quality

## Why Ollama

The selected local inference service is `Ollama`.

Reason:

- simple Windows install
- stable local service model
- OpenAI-compatible HTTP API
- low operational overhead compared with heavier inference stacks
- fits the existing `api` provider path without a new runtime architecture

This means local models use the same broad provider branch as external API
backends.

Runtime provider categories remain:

- `claude-cli`
- `api`

The local model path is an `api` provider configuration pointed at local
loopback.

## Supported Entry Points

There are now three operator-facing launcher scripts.

### 1. Claude CLI

File:

- `scripts/start-spire2mind-claude-cli.ps1`

Use when:

- you want the local logged-in Claude Code CLI path

### 2. External API

File:

- `scripts/start-spire2mind-api.ps1`

Use when:

- you want an external hosted backend
- you have a remote compatible API endpoint and model key

### 3. Local LLM API

File:

- `scripts/start-spire2mind-local-llm.ps1`

Use when:

- you want to run a local Ollama-backed model
- you want the simplest local operator entry point

This script:

- ensures Ollama is installed and reachable
- targets the local loopback API
- defaults to `qwen3:8b`
- can pre-pull models when needed

## Current Provider Selection Logic

The provider auto-selection logic was changed during this work.

Current behavior:

1. If `SPIRE2MIND_MODEL_PROVIDER` is explicitly set, that explicit choice wins.
2. If no provider is forced and a usable `API_BASE_URL` is configured, use `api`.
3. Only if API is not configured, fall back to `claude-cli`.
4. Otherwise the runtime stays deterministic.

Important consequence:

- a configured API endpoint now takes precedence over a locally installed Claude
  CLI

This applies to both:

- remote external APIs
- local loopback APIs like Ollama

## Local API Rules

The runtime now accepts local loopback API usage without requiring an API key.

Accepted local patterns:

- `http://127.0.0.1:...`
- `https://127.0.0.1:...`
- `http://localhost:...`
- `https://localhost:...`

For remote APIs, an API key is still expected.

Additional API protocol hint:

- `SPIRE2MIND_API_PROVIDER=openai`

This is used to make the SDK speak the correct wire format for OpenAI-compatible
local services.

## Files Changed For This Work

Primary code and script changes landed in:

- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/agent/runtime.go`
- `internal/agent/doctor.go`
- `scripts/start-spire2mind-api.ps1`
- `scripts/start-spire2mind-local-llm.ps1`
- `docs/runbook.md`

## Validated Behavior

The following validation was completed during this integration pass.

### Normal case

- Ollama service reachable on `127.0.0.1:11434`
- `qwen3:8b` responds through the OpenAI-compatible endpoint
- `spire2mind doctor` succeeds in local API mode

### Edge case

- loopback API without key is accepted as valid model config
- provider selection now prefers API over Claude CLI when API is configured

### Test coverage

Validated with:

- `go test ./internal/config`
- `go test ./internal/config ./internal/agent`

Also validated by real doctor run:

- `scripts/doctor.ps1` with local API environment variables

## Known Current Behavior

The current local model setup is integrated and usable, but it should be
treated as an operational baseline rather than a final quality claim.

What is known:

- the local provider path is alive
- the runtime can initialize against the local model
- the current machine can host the chosen model sizes

What still needs empirical runtime evaluation:

- long-run decision quality versus Claude
- action latency under real combat and menu pressure
- whether local model behavior increases seam noise or invalid actions
- whether more choices should move from the LLM into deterministic/planner logic

## Recommended Next Work For Agent Developers

Agent developers should start from live runtime behavior rather than more
provider plumbing.

Recommended next steps:

1. Run `headless-smoke` and `long-soak` on the local LLM path.
2. Compare run cleanliness against Claude-backed sessions.
3. Inspect whether local model failures are mostly:
   - poor action choice
   - stale index use
   - overlong output
   - action formatting drift
4. Tighten prompts and output constraints before changing architecture.
5. Continue shrinking the set of turns that truly require the model.

The main development question is no longer:

- "Can Spire2Mind talk to a local model?"

The main question is now:

- "How good and stable is local-model-backed play over long unattended runs?"

## Operator Commands

### Start local LLM TUI

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\scripts\start-spire2mind-local-llm.ps1 -Mode tui
```

### Run local LLM smoke

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\scripts\start-spire2mind-local-llm.ps1 -Mode headless-smoke -Attempts 1
```

### Run local LLM soak

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\scripts\start-spire2mind-local-llm.ps1 -Mode long-soak -Attempts 3
```

### Pre-pull local models

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\scripts\start-spire2mind-local-llm.ps1 -Mode tui -PullModel
```

## Summary

Spire2Mind now has a working local LLM integration path on Windows through
Ollama and the existing `api` runtime provider.

This work is complete at the infrastructure level.

The next phase should focus on:

- runtime quality
- action reliability
- soak stability
- prompt discipline
- reducing unnecessary model dependence
