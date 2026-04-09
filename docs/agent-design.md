# Agent Design

## Goal

The Go runtime should be able to drive a live single-player run from the main menu or an existing save until the run ends.
Winning is optional.
Finishing the run without human intervention is the primary target.

## Commands

- `spire2mind doctor`
- `spire2mind play`
- `spire2mind play --headless`
- `spire2mind play --headless --attempts N`

## Runtime modes

### Deterministic-first mode

If no model configuration is present, the runtime still works.
It uses a deterministic policy for obvious actions:

- modal handling
- continue run or new run bootstrap
- map movement
- basic reward resolution
- chest, rest, shop, and selection flow
- combat fallback decisions

This mode is useful for local smoke tests and regression coverage.

### Model-backed mode

If model configuration is available, the runtime initializes our forked
`github.com/hunknownz/open-agent-sdk-go`.

Supported model backends:

- `api`
  Use `SPIRE2MIND_API_BASE_URL`, `SPIRE2MIND_API_KEY`, and `SPIRE2MIND_MODEL`
  or compatible `ANTHROPIC_*` variables.
- `claude-cli`
  Use a logged-in local Claude Code CLI session through the SDK's persistent
  `claude-cli` provider.

The model uses the same Bridge tools as deterministic mode.
There is no second write path.
If the provider repeatedly fails, the runtime backs off and then degrades to deterministic mode for the rest of the session.

## Tool set

The runtime exposes five tools to the model:

- `get_game_state`
- `get_available_actions`
- `act`
- `wait_until_actionable`
- `get_game_data`

Rules:

- only one gameplay write action per cycle
- only actions listed in `availableActions`
- never reuse stale indexes after state changes
- handle modal states before anything else

## Session control loop

For each cycle:

1. Read the current actionable state.
2. If a forced rule matches, execute it immediately.
3. Otherwise, ask the model when it is available.
4. Execute exactly one action.
5. Persist the new state, artifacts, and event timeline.
6. Stop cleanly at `GAME_OVER` when the configured attempt limit is reached.
7. Otherwise, clear the game-over flow and begin the next attempt.

## Long-run helpers

The runtime already includes the long-cycle helpers from the design plan:

- `TodoManager`
  Maintains the current goal, room goal, recent failure, and next intent.
- `SkillLibrary`
  Loads screen-specific local knowledge from `data/skills/`.
- `CompactMemory`
  Shrinks prompt state when the session grows.
- `RunStore`
  Persists prompts, cycle summaries, latest state, and a full event stream.

These helpers stay outside the Bridge protocol.

Reflection memory is now carried forward across runtime restarts:

- the runtime loads recent `attempt-reflections.jsonl` files from prior artifacts
- those lessons and carry-forward plans are injected into the next session prompt
- the dashboard mirror also shows the current goals, room detail, and carry-forward memory
- recent run artifacts also contribute to a living seen-content codex so later runs know
  which cards, relics, monsters, events, potions, and characters have already appeared

## Waiting strategy

The runtime waits for the next actionable state in this order:

1. immediate `/state` check
2. SSE via `/events/stream`
3. polling fallback

This keeps the headless loop fast without making SSE a hard dependency.
Only states with real `availableActions` count as actionable.
An empty `GAME_OVER` screen is still a transition state and must not wake the model loop early.

## Artifacts

Each run creates a directory under `scratch/agent-runs/` that contains:

- `events.jsonl`
- `state-latest.json`
- `cycle-XXXX-prompt.txt`
- `cycle-XXXX-summary.json`
- `session-latest.json`
- `attempt-reflections.jsonl`
- `seen-content.json`
- `run-index.sqlite`

These artifacts are the main debugging surface for long unattended runs.
The session event stream now also records attempt numbers, provider fallback status, and chained-attempt progress.
The SQLite index mirrors session, attempt, cycle, recovery, reflection, and seen-content
data so long soaks can be queried without reparsing markdown and JSONL files.

In addition to the per-run directory, the runtime now refreshes a cross-run knowledge layer
under `scratch/guidebook/`:

- `guidebook.md`
- `living-codex.json`

These global artifacts aggregate recent runs into a living codex, recovery hotspot summary,
and merged strategy lessons.
