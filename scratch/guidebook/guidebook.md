# Spire2Mind Guidebook

This guidebook aggregates recent autonomous runs into a living codex, a recovery report, and practical lessons the agent can keep improving with.

## Overview

- Updated: `2026-04-14T07:58:13.563757Z`
- Runs scanned: `9`
- Reflections scanned: `4`

## RL Readiness

- Ready: `false`
- Status: not ready yet: complete runs 4/100, floor>=15 runs 1/20, provider-backed runs 4/60, recent clean runs 1/4, stable runtime=false
- Complete runs: `4 / 100`
- Floor >= 15 runs: `1 / 20`
- Provider-backed runs: `4 / 60`
- Recent clean runs: `1 / 4`
- Stable runtime window: `false`
- Knowledge assets ready: `true`

### Run Data Quality

- Clean complete runs: `1`
- Recent complete runs: `4`
- Recent provider-backed runs: `4`
- Recent fallback runs: `2`
- Recent provider-retry runs: `1`
- Recent tool-error runs: `1`
- Recent median floor: `8`
- Recent best floor: `15`
- Recent floor >= 7 runs: `4 / 4`
- Recent Act 2 entry runs: `0 / 4`
- Recent died-with-gold runs: `2`
- Recent average death gold: `126`

## Living Codex

- 已见卡牌: `79`
- 已见遗物: `16`
- 已见药水: `13`
- 已见怪物: `16`
- 已见事件: `6`
- 已见角色: `9`

- Recent discoveries:
  - 卡牌: Battle Trance (层数 `8`)
  - 卡牌: Pommel Strike (层数 `8`)
  - 卡牌: Demon Form (层数 `6`)
  - 怪物: Assassin Raider (层数 `6`)
  - 怪物: Axe Raider (层数 `6`)
  - 怪物: Brute Raider (层数 `6`)
  - 卡牌: Bully (层数 `5`)
  - 卡牌: Thunderclap (层数 `4`)
  - 卡牌: Peck (层数 `3`)
  - 事件: Wood Carvings (层数 `3`)

## Stable Heuristics

- Merged lessons:
  - Combat survival:
    - Play safer at low health: value block over damage and stop spending HP to race.
  - Pathing:
    - At low health, bias toward shorter and safer routes with rest or lower-variance rooms.
  - Reward choice:
    - Defense critically low: only 1 defense cards vs 6 attack cards. Prioritize block cards at next reward.
    - The midgame boss requires scaling or burst - pick at least one strong damage source before floor 17.
  - Shop economy:
    - Convert gold earlier: prioritize card removal, strong relics, or key shop cards before gold becomes useless.
    - Died with 156 gold and 6 basic Strikes still in deck. Should have removed Strikes at shop to improve draw quality.
  - Runtime:
    - Re-read the live state after fast transitions before repeating an indexed action.

## Recovery Hotspots

- Recent window: `6`

### Recent Recovery Hotspots

- recoverable: `240`
- invalid_action_rebind: `4`
- soft_replan / action_window_changed: `4`
- invalid_action: `2`
- decision_remap / same_screen_index_drift: `1`
- provider_retry: `1`

### Recency-Weighted Recovery Trends

- recoverable: `43.14`
- invalid_action_rebind: `1.25`
- soft_replan / action_window_changed: `1.25`
- decision_remap / same_screen_index_drift: `0.50`
- invalid_action: `0.50`
- provider_retry: `0.25`

### Historical Recovery Hotspots

- recoverable: `262`
- invalid_action_rebind: `4`
- soft_replan / action_window_changed: `4`
- invalid_action: `2`
- decision_remap / same_screen_index_drift: `1`
- provider_retry: `1`
- soft_replan / same_screen_index_drift: `1`
- soft_replan / screen_transition:game_over: `1`

Recent hotspots show what the latest runs are still tripping over; weighted trends keep historical context without letting old bugs dominate the signal.

## Failure Patterns

- A same-screen index drift changed the legal card/option indexes mid-turn.
- The combat resolved into GAME_OVER before the queued action landed; close out the run cleanly and bootstrap the next attempt.
- Fast transitions caused friction that forced runtime recovery instead of clean flow
- Died at critical HP (0/80) — the run collapsed with no safety margin left
- Died at critical HP (0/70) — the run collapsed with no safety margin left
- Died with 97 unspent gold — convert resources into survivability or power earlier
- Died at critical HP (0/1) — the run collapsed with no safety margin left
- Died with 156 unspent gold — convert resources into survivability or power earlier

## Recent Attempts

- Attempt 4 / BYGUT8SWKK
  - Outcome: defeat
  - Floor: `9`
  - Character: `IRONCLAD`
  - Headline: GAME_OVER: 1 available actions
  - Next plan: 下一周目请重点关注第 5-7 层的经济分配，必须在进入最终层之前完成核心牌组的关键移除与强化。保留至少 50 金币作为应对 Boss 的保险，并在血量跌破 50 时自动切换至防御姿态，拒绝高风险输出。
- Attempt 3 / ZF7X7P8JVG
  - Outcome: defeat
  - Floor: `8`
  - Character: `REGENT`
  - Headline: GAME_OVER: 1 available actions
  - Next plan: Stabilize the next few rooms before taking greedy lines, and Play safer at low health: value block over damage and stop spending HP to race
- Attempt 2 / QJ20AUAHPF
  - Outcome: defeat
  - Floor: `8`
  - Character: `SILENT`
  - Headline: GAME_OVER: 1 available actions
  - Next plan: Stabilize the next few rooms before taking greedy lines, double-check state after fast transitions and replan instead of repeating stale indexed actions, and Play safer at low h...
- Attempt 1 / Z46KXW4C1C
  - Outcome: defeat
  - Floor: `15`
  - Character: `IRONCLAD`
  - Headline: GAME_OVER: 1 available actions
  - Next plan: Stabilize the next few rooms before taking greedy lines, double-check state after fast transitions and replan instead of repeating stale indexed actions, and Play safer at low h...

## Story Seeds

- Attempt 1 on floor 15: Fast transitions caused friction that forced runtime recovery instead of clean flow
- Attempt 2 on floor 8: Fast transitions caused friction that forced runtime recovery instead of clean flow
- Attempt 3 on floor 8: Died at critical HP (0/1) — the run collapsed with no safety margin left
- Attempt 4 on floor 9: Died at critical HP (0/80) — the run collapsed with no safety margin left