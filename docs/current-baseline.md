# Current Baseline

## Scope

This document captures the current Spire2Mind baseline after the system moved past
initial infrastructure bring-up and into long-run stability work.

The project is no longer a bridge prototype.
It is now a live automation harness with:

- a game-resident Bridge as the only writer
- a Go runtime that owns the control loop
- deterministic, planner, and model-backed decision layers
- persistent artifacts and cross-run knowledge assets

The primary near-term goal is not feature breadth.
It is stable unattended play across many runs with data quality high enough to
justify later RL work.

## Operational Entry Points

Preferred launcher scripts now live under `scripts/` and should be used instead
of ad hoc shell commands.

- `scripts/start-spire2mind-claude-cli.ps1`
  - provider-fixed entry point for Claude Code CLI mode
  - supports `tui`, `headless-smoke`, and `long-soak`
- `scripts/start-spire2mind-api.ps1`
  - provider-fixed entry point for API mode
  - supports `tui`, `headless-smoke`, and `long-soak`

Both wrappers stop existing local Spire2Mind launcher/play processes before
starting a new run, so local multi-instance conflicts are avoided by default.

## Architecture

### Bridge

The Bridge runs inside the game process and is the only component allowed to
touch live game objects.

Responsibilities:

- read live game state on the Godot main thread
- expose a stable HTTP/SSE API
- normalize raw gameplay/UI state into a usable schema
- execute validated actions on the game thread
- wait for settled post-action state before returning

Core API:

- `GET /health`
- `GET /state`
- `GET /state?format=markdown`
- `GET /actions/available`
- `POST /action`
- `GET /events/stream`

### Harness runtime

The Go runtime owns the single gameplay loop:

1. read a fresh actionable state
2. choose a rule/planner/model action
3. execute exactly one action through the Bridge
4. wait for the next actionable state
5. persist artifacts, summaries, and reflections

The runtime never writes directly into the game.

### Decision stack

The runtime is intentionally layered:

- deterministic rules for modal flow, obvious transitions, and single-action states
- planner support for combat and shortlist-style decision shaping
- model-backed decisions for ambiguous or strategic choices

The current model backend is our maintained
`github.com/hunknownz/open-agent-sdk-go`, primarily through the persistent
`claude-cli` provider.

### Knowledge and memory

The project now has four knowledge layers:

- static truth: `data/eng`
- per-run artifacts: `scratch/agent-runs/...`
- cross-run guidebook/codex assets:
  - `scratch/guidebook/guidebook.md`
  - `scratch/guidebook/living-codex.json`
  - `scratch/guidebook/combat-playbook.md`
  - `scratch/guidebook/event-playbook.md`
- prompt-time working memory:
  - `TodoManager`
  - `CompactMemory`
  - `SkillLibrary`

### Observability

The main operator surfaces are:

- visible TUI via `spire2mind play`
- unattended validation via `spire2mind play --headless`
- persisted `dashboard.md`
- per-run `events.jsonl`
- global `guidebook.md`

Reverse-engineering and seam findings live in:

- `docs/bridge-reverse-engineering.md`
- `docs/bridge-validation-matrix.md`

## Current Readiness

Snapshot as of `2026-04-07` from `scratch/guidebook/guidebook.md`:

- runs scanned: `24`
- complete runs: `18 / 100`
- floor >= 15 runs: `3 / 20`
- provider-backed runs: `16 / 60`
- recent clean runs: `0 / 4`
- stable runtime: `false`
- knowledge assets ready: `true`

Interpretation:

- The system already produces enough data to justify richer codex semantics and
  planner priors.
- The system does not yet produce stable-enough data to begin RL training.
- The bottleneck has shifted from capability coverage to runtime noise and run quality.

## What Already Works

- Bridge-driven play through menu, character select, map, combat, rewards,
  events, chest, shop, rest, selection, and game over flow
- SSE-first waiting with polling fallback
- model-backed combat and non-combat choices through Claude CLI
- planner-backed combat analysis with MCTS-style shallow search
- continuous autoplay with chained attempts
- persistent run artifacts and cross-run knowledge aggregation

## Main Bottlenecks

Current recovery hotspots show the same pattern repeatedly:

- `soft_replan / action_window_changed`
- `soft_replan / same_screen_index_drift`
- `decision_remap / same_screen_index_drift`
- `soft_replan / screen_transition:combat`
- `invalid_action`
- `soft_replan / reward_transition`

These are not missing features.
They are state-settling and action-contract quality problems.

The most important practical consequence is that recent runs still contain too
much runtime recovery noise to count as RL-grade data.

## Current Design Direction

The next phase should continue to respect the existing architecture:

- no second game control path
- no bypass around the Bridge
- no RL-first detour
- no MCP-style handoff rewrite

The right direction is:

1. tighten invariants and action contracts
2. reduce runtime seam noise
3. improve planner quality on stable turns
4. accumulate cleaner provider-backed runs
5. start RL only when the data quality gates are met

## Recommended Next Steps

### 1. Continue turning runtime failures into invariants

This is the highest-value work.

Use the STS2-Agent lesson directly:

- define state/action contracts
- encode them as tests
- only then add runtime recovery if necessary

Immediate targets:

- combat `action_window_changed`
- `same_screen_index_drift`
- reward transition edges
- post-combat and game-over phase boundaries

### 1.5 Replace patch chains with explicit runtime subsystems

The current runtime is far enough along that recurring seam fixes should now land as
subsystems instead of helper growth.

The active redesign order is:

- `ActionResolutionPipeline`
  - one path for action remap, normalization, and validation
- `StableStateGate`
  - one path for fresh/stable/actionable state reads
- `PromptAssemblyPipeline`
  - screen-scoped prompt blocks instead of full-state prompt accretion
- `Reasoning/Reflection Pipeline`
  - separate live action rationale from attempt-level learning artifacts

The first subsystem is now in progress and is the correct place to keep retiring:

- `play_card invalid_target`
- `same_screen_index_drift`
- scattered request normalization

### 2. Shrink the set of actions that require the model

The system should continue moving decisions out of the LLM path when the answer
is procedural or structurally obvious.

That improves:

- stability
- latency
- token use
- training data quality

### 3. Keep improving MCTS, but only as a planner

MCTS should remain a harness-internal combat planner.

Near-term planner work:

- reduce overblocking on low-pressure turns
- improve offensive tempo bias when survival is already covered
- consume codex semantics only through structured tags and evidence, not free text

### 4. Keep strengthening the guidebook/codex pipeline

The current asset split is correct.
The next step is better evidence quality, not more file types.

The codex should keep moving from:

- "what was seen"

to:

- "what tends to be risky"
- "how the agent should respond"
- "what failure patterns co-occur with this entity"

### 5. Delay RL until the gates are actually met

RL should only begin after all of the following are true:

- `100+` complete runs
- `20+` runs that reached floor `>= 15`
- `60+` provider-backed complete runs
- `4+` recent clean runs
- recent runtime window passes the stability gate

The stability gate is currently defined in code by:

- low recent `reward_transition`
- low recent `same_screen_index_drift`
- low recent `selection_seam`
- zero recent fallback/provider-retry/tool-error noise

## Practical Short-Term Priority Order

The current implementation order should stay:

1. inspect `stdout.log + events.jsonl`
2. inspect `guidebook.md`
3. patch the highest-value hotspot
4. run regression tests
5. restart single-instance continuous TUI/soak

For the current baseline, the best next technical targets are:

1. `action_window_changed`
2. `same_screen_index_drift`
3. `reward_transition`
4. combat planner over-defensive lines
5. provider-backed long-run cleanliness

## Summary

Spire2Mind is currently a stable harness-first automation system with real
knowledge accumulation and a functioning planner/model stack.

The project does not need a new architecture.
It needs cleaner runtime behavior, more provider-backed complete runs, and
continued disciplined conversion of recurring seams into contracts and tests.
