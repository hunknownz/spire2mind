# 战斗作战手册

这份手册从最近的自主 runs 里提炼战斗侧经验，用来约束战术规划、目标选择和低血量时的保命纪律。

## 概览

- 更新时间: `2026-04-14T07:58:13.563757Z`
- 扫描 runs: `9`
- 扫描反思: `4`

## 战斗优先级

- 生存:
  - Play safer at low health: value block over damage and stop spending HP to race.
- 奖励选择:
  - Defense critically low: only 1 defense cards vs 6 attack cards. Prioritize block cards at next reward.
  - The midgame boss requires scaling or burst - pick at least one strong damage source before floor 17.

## 已观测怪物

- 怪物: Leaf Slime (M) (出现 `8`, 层数 `8`)
- 怪物: Slithering Strangler (出现 `8`, 层数 `8`)
- 怪物: Fuzzy Wurm Crawler (出现 `8`, 层数 `5`)
- 怪物: Leaf Slime (S) (出现 `8`, 层数 `3`)
- 怪物: Twig Slime (S) (出现 `8`, 层数 `3`)
- 怪物: Snapping Jaxfruit (出现 `8`, 层数 `8`)
- 怪物: Twig Slime (M) (出现 `8`, 层数 `4`)
- 怪物: Byrdonis (出现 `8`, 层数 `15`)
- 怪物: Mawler (出现 `8`, 层数 `5`)
- 怪物: Nibbit (出现 `3`, 层数 `4`)
- 怪物: Wriggler (出现 `2`, 层数 `9`)
- 怪物: Phrog Parasite (出现 `2`, 层数 `9`)

## 已观测战斗资源

- 卡牌: Bash (出现 `8`, 层数 `9`)
- 卡牌: Defend (出现 `8`, 层数 `9`)
- 卡牌: Headbutt (出现 `8`, 层数 `9`)
- 卡牌: Strike (出现 `8`, 层数 `9`)
- 卡牌: True Grit (出现 `8`, 层数 `9`)
- 卡牌: Bloodletting (出现 `8`, 层数 `8`)
- 卡牌: Slimed (出现 `8`, 层数 `8`)
- 卡牌: Setup Strike (出现 `8`, 层数 `6`)
- 遗物: Bag of Marbles (出现 `8`, 层数 `12`)
- 遗物: Oddly Smooth Stone (出现 `8`, 层数 `12`)
- 遗物: The Abacus (出现 `8`, 层数 `12`)
- 遗物: Blood Vial (出现 `8`, 层数 `10`)
- 遗物: Akabeko (出现 `8`, 层数 `9`)
- 遗物: Dingy Rug (出现 `8`, 层数 `9`)

## 战斗失败模式

- The combat resolved into GAME_OVER before the queued action landed; close out the run cleanly and bootstrap the next attempt.
- Died at critical HP (0/80) — the run collapsed with no safety margin left
- Died at critical HP (0/70) — the run collapsed with no safety margin left
- Died with 97 unspent gold — convert resources into survivability or power earlier
- Died at critical HP (0/1) — the run collapsed with no safety margin left
- Died with 156 unspent gold — convert resources into survivability or power earlier

## 战斗接缝观察

- soft_replan / action_window_changed: `1.25`
- decision_remap / same_screen_index_drift: `0.50`

## 近期故事种子

- Attempt 1 on floor 15: Fast transitions caused friction that forced runtime recovery instead of clean flow
- Attempt 2 on floor 8: Fast transitions caused friction that forced runtime recovery instead of clean flow
- Attempt 3 on floor 8: Died at critical HP (0/1) — the run collapsed with no safety margin left
- Attempt 4 on floor 9: Died at critical HP (0/80) — the run collapsed with no safety margin left