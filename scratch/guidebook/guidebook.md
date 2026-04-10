# Spire2Mind Guidebook

This guidebook aggregates recent autonomous runs into a living codex, a recovery report, and practical lessons the agent can keep improving with.

## Overview

- Updated: `2026-04-10T04:35:09.5138894Z`
- Runs scanned: `19`
- Reflections scanned: `2`

## RL Readiness

- Ready: `false`
- Status: not ready yet: complete runs 2/100, floor>=15 runs 0/20, provider-backed runs 2/60, recent clean runs 2/4, stable runtime=false
- Complete runs: `2 / 100`
- Floor >= 15 runs: `0 / 20`
- Provider-backed runs: `2 / 60`
- Recent clean runs: `2 / 4`
- Stable runtime window: `false`
- Knowledge assets ready: `true`

### Run Data Quality

- Clean complete runs: `2`
- Recent complete runs: `2`
- Recent provider-backed runs: `2`
- Recent fallback runs: `0`
- Recent provider-retry runs: `0`
- Recent tool-error runs: `0`
- Recent median floor: `7`
- Recent best floor: `7`
- Recent floor >= 7 runs: `2 / 2`
- Recent Act 2 entry runs: `0 / 2`
- Recent died-with-gold runs: `2`
- Recent average death gold: `122`

## Living Codex

- 已见卡牌: `34`
- 已见遗物: `3`
- 已见药水: `3`
- 已见怪物: `15`
- 已见事件: `2`
- 已见角色: `7`

- Recent discoveries:
  - 卡牌: Forgotten Ritual (层数 `7`)
  - 卡牌: Stone Armor (层数 `7`)
  - 药水: Colorless Potion (层数 `5`)
  - 卡牌: Fasten (层数 `5`)
  - 卡牌: Flame Barrier (层数 `5`)
  - 药水: Flex Potion (层数 `5`)
  - 卡牌: Hand of Greed (层数 `5`)
  - 遗物: Lantern (层数 `5`)
  - 药水: Skill Potion (层数 `5`)
  - 遗物: Stone Calendar (层数 `5`)

## Stable Heuristics

- Merged lessons:
  - Combat survival:
    - Play safer at low health: value block over damage and stop spending HP to race.
  - Pathing:
    - At low health, bias toward shorter and safer routes with rest or lower-variance rooms.
  - Shop economy:
    - Convert gold earlier: prioritize card removal, strong relics, or key shop cards before gold becomes useless.

## Recovery Hotspots

- Recent window: `6`

### Recent Recovery Hotspots

- hard_replan / same_screen_state_drift: `1`
- invalid_action: `1`

### Recency-Weighted Recovery Trends

- hard_replan / same_screen_state_drift: `0.50`
- invalid_action: `0.50`

### Historical Recovery Hotspots

- hard_replan / same_screen_state_drift: `1`
- invalid_action: `1`

Recent hotspots show what the latest runs are still tripping over; weighted trends keep historical context without letting old bugs dominate the signal.

## Failure Patterns

- Died at critical HP (0/80) — the run collapsed with no safety margin left
- Died with 82 unspent gold — convert resources into survivability or power earlier
- Died with 163 unspent gold — convert resources into survivability or power earlier

## Recent Attempts

- Attempt 2 / A6RYL65D6T
  - Outcome: defeat
  - Floor: `7`
  - Character: `IRONCLAD`
  - Headline: GAME_OVER: 1 available actions
  - Next plan: Stabilize the next few rooms before taking greedy lines, and Play safer at low health: value block over damage and stop spending HP to race
- Attempt 1 / L2ZHQ4MU8Y
  - Outcome: defeat
  - Floor: `7`
  - Character: `IRONCLAD`
  - Headline: GAME_OVER: 1 available actions
  - Next plan: Stabilize the next few rooms before taking greedy lines, and Play safer at low health: value block over damage and stop spending HP to race

## Story Seeds

- Attempt 1 on floor 7: Died at critical HP (0/80) — the run collapsed with no safety margin left
- Attempt 2 on floor 7: Died at critical HP (0/80) — the run collapsed with no safety margin left