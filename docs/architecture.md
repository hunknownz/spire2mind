# Spire2Mind Architecture

## Goal

Spire2Mind runs a live Slay the Spire 2 automation stack on Windows:

- a C# Bridge mod runs inside the game process
- a Go runtime talks to the Bridge over HTTP and SSE
- a deterministic policy and optional model-driven agent choose actions
- a headless runner and a Bubble Tea TUI share the same control loop

The system is designed so the Bridge remains the only writer to the game.

Current phase status and near-term priorities are tracked in
[current-baseline.md](/C:/Users/klerc/spire2mind/docs/current-baseline.md).

## Layers

### 1. Game + Bridge

The Bridge is a signed mod loaded by the Steam Windows build of Slay the Spire 2.
It exposes the live game state through HTTP and executes actions on the game thread.

Key responsibilities:

- read live game objects on the Godot main thread
- normalize gameplay state into a stable JSON schema
- expose available actions derived from the live UI and game state
- execute legal actions and wait for the next stable snapshot
- publish state changes through SSE

### 2. Go runtime

The Go runtime is the harness layer.
It never writes directly into the game process.
It only uses Bridge tools:

- `get_game_state`
- `get_available_actions`
- `act`
- `wait_until_actionable`
- `get_game_data`

The runtime supports two modes:

- deterministic-only mode when no model configuration is present
- model-backed mode through our maintained `github.com/hunknownz/open-agent-sdk-go`
  - `api` for Anthropic-compatible Messages API backends
  - `claude-cli` for a persistent local Claude Code CLI session

When the model provider repeatedly fails, the runtime degrades to deterministic mode instead of abandoning the live run immediately.

### 3. Strategy helpers

The harness keeps lightweight long-run state outside the model:

- `TodoManager` for short-term objectives
- `SkillLibrary` for screen-specific local knowledge
- `CompactMemory` for prompt compaction and cross-attempt lessons
- `RunStore` for transcripts, latest state, prompts, per-cycle summaries, dashboard snapshots, and attempt reflections
- `RunStore` also mirrors structured session data into a per-run SQLite index for
  attempts, recoveries, reflections, cycle summaries, and seen-content discoveries

These helpers guide the model but do not bypass the Bridge or create a second gameplay protocol.

### 4. Operator interfaces

- `spire2mind play --headless`
  Main unattended validation path.
- `spire2mind play`
  Bubble Tea UI with live status, current screen, actions, logs, and run metadata.
- `dashboard.md`
  A persisted mirror of the live debug view for environments where the terminal UI is not directly visible.
- `spire2mind doctor`
  Environment, Bridge, SSE, data, and model health checks.

## Runtime flow

1. The game starts and loads the Bridge mod.
2. The Bridge opens an HTTP listener on `127.0.0.1:8080` by default.
3. The Go runtime reads `/state` and `/actions/available`.
4. If a deterministic rule matches, it executes one action immediately.
5. Otherwise, the model can inspect tools and choose one legal action.
6. After every action, the runtime waits for the next actionable state by SSE first and polling second.
7. The loop continues until the configured attempt budget is exhausted, an unrecoverable failure occurs, or a bounded safety stop triggers.

## Reliability principles

- Single writer: only the Bridge sends write actions into the game.
- Main-thread safety: all game reads and writes pass through `GameThread`.
- Stable snapshots: actions only report `completed` after the next settled state.
- Transition suppression: map travel and other transient states do not expose ghost actions.
- Fallbacks: polling still works when SSE is unavailable.
- Provider resilience: repeated provider failures trigger bounded retries and a deterministic fallback path.
- Local artifacts: every run stores a timeline, latest state, and cycle summaries under `scratch/agent-runs/`.
- Formal run index: each run also writes `run-index.sqlite`, which is the query-friendly
  mirror of the session event stream and world codex snapshot for later analysis,
  guide generation, and future training pipelines.
- Global knowledge layer: recent runs are also merged into `scratch/guidebook/guidebook.md`
  and `scratch/guidebook/living-codex.json`, which track discovered content, merged lessons,
  and recovery hotspots across attempts.

## Current scope

Implemented and actively validated:

- main menu and save resume
- character select and embark
- map movement
- combat
- reward resolution
- event resolution
- chest resolution
- rest site resolution
- shop inventory, purchases, and card removal
- selection screens
- SSE event stream
- deterministic headless play across multiple rooms
- chained attempt support through game over recovery actions

Deferred:

- multiplayer support
- timeline features
- customer distribution signing
- any non-Bridge control path
