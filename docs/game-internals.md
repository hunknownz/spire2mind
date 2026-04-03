# 杀戮尖塔 2 游戏内部结构参考

本文档基于 STS2-Agent 项目的逆向工程成果整理，供 spire2mind 开发过程中查阅和排查问题使用。

游戏版本基线：`v0.98.2`（后续游戏更新需要重新验证）。

---

## 1. 游戏技术栈

| 项目 | 值 |
|------|----|
| 引擎 | Godot 4.5.1 |
| 语言运行时 | .NET 9.0（`Microsoft.NETCore.App 9.0.7`） |
| C# 绑定 | `GodotSharp.dll 4.5.1.0` |
| Mod 补丁库 | `0Harmony.dll 2.4.2.0` |
| HTTP 支持 | `System.Net.HttpListener.dll`（游戏目录内自带） |

**关键推论**：
- 游戏不是 Unity + BepInEx，而是 Godot + .NET
- Harmony patch 可行（游戏已自带 0Harmony.dll）
- HttpListener 可行（游戏已自带依赖）

## 2. 游戏目录结构

```
Slay the Spire 2/
├── SlayTheSpire2.exe                    ← 游戏启动器
├── steam_appid.txt                      ← Steam AppID: 2868840
├── release_info.json                    ← 版本信息
├── data_sts2_windows_x86_64/            ← Windows 程序集目录
│   ├── sts2.dll                         ← 游戏核心程序集（约 8.9MB）
│   ├── GodotSharp.dll                   ← Godot C# 绑定
│   ├── 0Harmony.dll                     ← Harmony 补丁库
│   └── System.Net.HttpListener.dll      ← HTTP 支持
├── data_sts2_osx_arm64/                 ← macOS ARM 程序集目录
├── data_sts2_osx_x86_64/               ← macOS Intel 程序集目录
└── mods/                                ← Mod 安装目录
    ├── Spire2Mind.Bridge.dll
    ├── Spire2Mind.Bridge.pck
    └── mod_id.json
```

## 3. Mod 加载机制

通过反编译 `sts2.dll` 中的 `ModManager` 确认的加载流程：

### 加载步骤

```
游戏启动
  → ModManager 扫描 mods/ 目录
  → 递归查找 .pck 文件
  → 检查 settings.save 中 mod_settings.mods_enabled == true
  → AssemblyLoadContext.LoadFromAssemblyPath() 加载同名 .dll
  → ProjectSettings.LoadResourcePack() 加载 .pck
  → 扫描程序集中的 [ModInitializer] 属性
  → 调用标注的静态方法
```

### 关键约束

| 约束 | 说明 |
|------|------|
| 扫描目录 | `Path.Combine(directoryName, "mods")` |
| DLL 位置 | 与 .pck 同目录、同名 |
| 包内要求 | .pck 必须包含 `res://mod_manifest.json` |
| 清单字段 | `pck_name`、`name`、`author`、`description`、`version` |
| 名称约束 | `mod_manifest.json` 中的 `pck_name` 必须和 .pck 文件名一致 |
| 用户门槛 | `SaveManager.Instance.SettingsSave.ModSettings?.PlayerAgreedToModLoading` 必须为 true |

### Mod 设置持久化

| 项目 | 值 |
|------|---|
| 设置文件 | `%APPDATA%/SlayTheSpire2/default/1/settings.save` |
| 文件格式 | 明文 JSON |
| 启用字段 | `mod_settings.mods_enabled` |

**首次加载问题**：如果日志出现 `Skipping loading mod ... user has not yet seen the mods warning`，需要在 settings.save 中设置 `"mod_settings": { "mods_enabled": true }`。

### Mod 初始化

| 项目 | 说明 |
|------|------|
| 属性类型 | `MegaCrit.Sts2.Core.Modding.ModInitializerAttribute` |
| 作用目标 | 加在类上 |
| 传入参数 | 初始化方法名字符串 |
| 方法要求 | static，允许 public 或 non-public |
| 回退路径 | 如果没有 ModInitializerAttribute，则执行 Harmony.PatchAll(assembly) |

## 4. 核心游戏对象

### 单例入口

| 对象 | 命名空间 | 用途 |
|------|---------|------|
| `RunManager.Instance` | `MegaCrit.Sts2.Core.Runs` | 运行状态、动作队列、存档 |
| `CombatManager.Instance` | `MegaCrit.Sts2.Core.Combat` | 战斗管理、回合控制 |
| `ActiveScreenContext.Instance` | `MegaCrit.Sts2.Core.Nodes.Screens.ScreenContext` | 当前画面 |
| `LocalContext` | `MegaCrit.Sts2.Core.Context` | 本地玩家获取 |
| `NGame.Instance` | `MegaCrit.Sts2.Core.Nodes` | Godot 场景树访问 |

### 状态获取

```csharp
// 战斗状态
var combatState = CombatManager.Instance.DebugOnlyGetState();

// 运行状态
var runState = RunManager.Instance.DebugOnlyGetState();

// 当前画面
var currentScreen = ActiveScreenContext.Instance.GetCurrentScreen();

// 本地玩家
var me = LocalContext.GetMe(combatState);
var me = LocalContext.GetMe(runState);
```

### 画面判定优先级

`ActiveScreenContext.GetCurrentScreen()` 按以下顺序检查：

```
1. FeedbackScreen
2. OpenModal                         ← Modal 弹窗最高优先级
3. 检视卡牌/遗物屏幕
4. 主菜单及子菜单 (NMainMenu)
5. NMapScreen
6. NOverlayStack
7. EventRoom (NEventRoom)
8. CombatRoom (NCombatRoom)
9. TreasureRoom (NTreasureRoom)
10. RestSiteRoom (NRestSiteRoom)
11. MapRoom (NMapRoom)
12. MerchantRoom (NMerchantRoom)
```

**设计启示**：`/state.screen` 应基于 `ActiveScreenContext` + 房间对象，而不是简化枚举。

## 5. 动作执行机制

### 出牌

```
调用链：
  CardModel.TryManualPlay(Creature? target)
    → CardModel.EnqueueManualPlay(target)
      → RunManager.Instance.ActionQueueSynchronizer.RequestEnqueue(
          new PlayCardAction(this, target))
```

### 结束回合

```csharp
RunManager.Instance.ActionQueueSynchronizer.RequestEnqueue(
    new EndPlayerTurnAction(me, roundNumber));
```

### 状态稳定判定

等待机制：监听 `ActionExecutor.AfterActionExecuted`，直到没有 ready 的 player-driven action。

候选等待点：`CombatManager.WaitUntilQueueIsEmptyOrWaitingOnNonPlayerDrivenAction()`

实际实现（推荐）：yield 到 Godot 帧信号做协作式等待：

```csharp
while (DateTime.UtcNow < deadline)
{
    await NGame.Instance.ToSignal(
        NGame.Instance.GetTree(), SceneTree.SignalName.ProcessFrame);
    if (isStable()) return true;
}
```

### 地图推进

```
地图动作类：MoveToMapCoordAction(Player player, MapCoord destination)
非测试模式：调用 NMapScreen.Instance.TravelToMapCoord(_destination)
```

### 奖励后推进

```
RunManager.ProceedFromTerminalRewardsScreen()
→ 打开地图，或在战斗事件中恢复上一个房间
```

## 6. 关键命名空间索引

### Modding

| 类型 | 命名空间 |
|------|---------|
| ModManifest | `MegaCrit.Sts2.Core.Modding` |
| ModManager | `MegaCrit.Sts2.Core.Modding` |
| ModInitializerAttribute | `MegaCrit.Sts2.Core.Modding` |

### 战斗与动作

| 类型 | 命名空间 |
|------|---------|
| CombatManager | `MegaCrit.Sts2.Core.Combat` |
| RunManager | `MegaCrit.Sts2.Core.Runs` |
| Creature | `MegaCrit.Sts2.Core.Entities.Creatures` |
| Player | `MegaCrit.Sts2.Core.Entities.Players` |
| CardModel | `MegaCrit.Sts2.Core.Entities.Cards` |
| PlayCardAction | `MegaCrit.Sts2.Core.GameActions` |
| EndPlayerTurnAction | `MegaCrit.Sts2.Core.GameActions` |
| UsePotionAction | `MegaCrit.Sts2.Core.GameActions` |
| ActionQueueSynchronizer | `MegaCrit.Sts2.Core.Runs` |

### 场景与节点

| 类型 | 命名空间 |
|------|---------|
| NMapScreen | `MegaCrit.Sts2.Core.Nodes.Screens.Map` |
| NEventRoom | `MegaCrit.Sts2.Core.Nodes.Rooms` |
| NMerchantRoom | `MegaCrit.Sts2.Core.Nodes.Rooms` |
| NRestSiteRoom | `MegaCrit.Sts2.Core.Nodes.Rooms` |
| NTreasureRoom | `MegaCrit.Sts2.Core.Nodes.Rooms` |
| NCombatRoom | `MegaCrit.Sts2.Core.Nodes.Combat` |
| NRewardsScreen | `MegaCrit.Sts2.Core.Nodes.Rewards` |
| NRewardButton | `MegaCrit.Sts2.Core.Nodes.Rewards` |
| NIntent | `MegaCrit.Sts2.Core.Nodes.Combat` |
| NOverlayStack | `MegaCrit.Sts2.Core.Nodes.Screens.Overlays` |

### 流程候选入口

| 符号 | 说明 |
|------|------|
| `RunManager.<EnterMapPointInternal>` | 地图推进入口 |
| `RunManager.<ProceedFromTerminalRewardsScreen>` | 奖励结束后继续 |
| `RunManager.<EnterRoomInternal>` | 房间进入入口 |
| `CombatManager.<StartTurn>` | 玩家回合开始 |
| `CombatManager.<DoTurnEnd>` | 回合结束 |
| `CombatManager.<EndCombatInternal>` | 战斗结束 |

## 7. HTTP API 协议

### 端点

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /health | Mod 健康检查 |
| GET | /state | 完整游戏状态 JSON |
| GET | /state?format=markdown | Markdown 格式状态 |
| GET | /actions/available | 当前可用动作列表 |
| GET | /events/stream | SSE 事件流 |
| POST | /action | 执行游戏动作 |

### 响应格式

```json
// 成功
{ "ok": true, "request_id": "req_...", "data": { ... } }

// 失败
{ "ok": false, "request_id": "req_...", "error": {
    "code": "invalid_action", "message": "...", "details": {...}, "retryable": false
}}
```

### 错误码

| 错误码 | HTTP 状态码 | 含义 | 可重试 |
|--------|-----------|------|--------|
| invalid_request | 400 | 请求体缺少必要字段 | 否 |
| not_found | 404 | 路由不存在 | 否 |
| invalid_action | 409 | 当前状态不允许该动作 | 否 |
| invalid_target | 409 | 目标索引越界 | 否 |
| state_unavailable | 503 | 游戏状态暂时不可读 | 是 |
| internal_error | 500 | 内部异常 | 否 |

### Screen 枚举

| 值 | 含义 |
|----|------|
| MAIN_MENU | 主菜单 |
| CHARACTER_SELECT | 角色选择 |
| MAP | 地图 |
| COMBAT | 战斗中 |
| EVENT | 事件 |
| SHOP | 商店 |
| REST | 休息点 |
| REWARD | 奖励结算 |
| CHEST | 宝箱房 |
| CARD_SELECTION | 选牌界面 |
| MODAL | 弹窗 |
| GAME_OVER | 游戏结束 |
| UNKNOWN | 无法识别 |

### Action Status

| 值 | 含义 |
|----|------|
| completed | 动作完成，state 已稳定 |
| pending | 动作已提交，状态仍在过渡中 |

### 支持的动作

| 动作名 | 参数 | 说明 |
|--------|------|------|
| `play_card` | card_index, target_index? | 从手牌打出一张卡 |
| `end_turn` | 无 | 结束当前回合 |
| `use_potion` | option_index, target_index? | 使用药水 |
| `discard_potion` | option_index | 丢弃药水 |
| `choose_map_node` | option_index | 选择地图节点 |
| `collect_rewards_and_proceed` | 无 | 自动收集奖励并离开 |
| `claim_reward` | option_index | 领取单个奖励 |
| `choose_reward_card` | option_index | 选择卡牌奖励 |
| `skip_reward_cards` | 无 | 跳过卡牌奖励 |
| `select_deck_card` | option_index | 选牌界面选择一张卡 |
| `confirm_selection` | 无 | 确认多选 |
| `open_chest` | 无 | 打开宝箱 |
| `choose_treasure_relic` | option_index | 选择宝箱遗物 |
| `choose_event_option` | option_index | 选择事件选项 |
| `choose_rest_option` | option_index | 选择休息点操作 |
| `open_shop_inventory` | 无 | 打开商店库存 |
| `close_shop_inventory` | 无 | 关闭商店库存 |
| `buy_card` | option_index | 购买卡牌 |
| `buy_relic` | option_index | 购买遗物 |
| `buy_potion` | option_index | 购买药水 |
| `remove_card_at_shop` | 无 | 商店删牌服务 |
| `select_character` | option_index | 选择角色 |
| `embark` | 无 | 开始游戏 |
| `continue_run` | 无 | 继续存档 |
| `abandon_run` | 无 | 放弃当前运行 |
| `confirm_modal` | 无 | 确认弹窗 |
| `dismiss_modal` | 无 | 取消弹窗 |
| `return_to_main_menu` | 无 | 游戏结束后回主菜单 |
| `proceed` | 无 | 点击继续按钮 |

## 8. State 详细字段参考

### `/state` 顶层字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `state_version` | number | 状态模型版本 |
| `run_id` | string | 本局运行标识（种子字符串） |
| `screen` | string | 当前逻辑界面（见 Screen 枚举） |
| `in_combat` | boolean | 是否处于战斗 |
| `turn` | number \| null | 当前回合数（非战斗为 null） |
| `available_actions` | string[] | 当前可执行动作列表 |
| `combat` | object \| null | 战斗状态 |
| `run` | object \| null | 本局运行状态 |
| `map` | object \| null | 地图状态 |
| `reward` | object \| null | 奖励状态 |
| `selection` | object \| null | 选牌状态 |
| `chest` | object \| null | 宝箱状态 |
| `event` | object \| null | 事件状态 |
| `shop` | object \| null | 商店状态 |
| `rest` | object \| null | 休息点状态 |
| `character_select` | object \| null | 角色选择状态 |
| `modal` | object \| null | 弹窗状态 |
| `game_over` | object \| null | 游戏结束状态 |
| `agent_view` | object \| null | AI 精简视图 |

### `combat` 子结构

#### `combat.player`

| 字段 | 类型 | 说明 |
|------|------|------|
| `current_hp` | number | 当前生命值 |
| `max_hp` | number | 最大生命值 |
| `block` | number | 格挡值 |
| `energy` | number | 当前能量 |
| `stars` | number | 星星数 |
| `powers[]` | object[] | Power / Buff / Debuff 列表 |

#### `combat.player.powers[]`

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | number | 索引 |
| `power_id` | string | Power 内部 ID |
| `name` | string | 显示名称 |
| `amount` | number \| null | 层数/数值 |
| `is_debuff` | boolean | 是否为 Debuff |

#### `combat.hand[]`

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | number | 手牌索引（用于 `play_card` 的 `card_index`） |
| `card_id` | string | 卡牌内部 ID |
| `name` | string | 显示名称 |
| `upgraded` | boolean | 是否已升级 |
| `target_type` | string | 目标类型（`None`, `AnyEnemy`, `AnyAlly`） |
| `requires_target` | boolean | 是否需要指定目标 |
| `costs_x` | boolean | 是否为能量 X 费 |
| `star_costs_x` | boolean | 是否为星星 X 费 |
| `energy_cost` | number | 能量消耗（含修正） |
| `star_cost` | number | 星星消耗（含修正） |
| `playable` | boolean | 当前是否可打出 |
| `unplayable_reason` | string \| null | 不可打出原因 |
| `rules_text` | string | 原始规则文本 |
| `resolved_rules_text` | string | 动态变量展开后的规则文本 |
| `dynamic_values[]` | object[] | 动态变量列表 |

**unplayable_reason 已知值**：`not_enough_energy`、`not_enough_stars`、`no_living_allies`、`blocked_by_hook`、`unplayable`

#### `combat.enemies[]`

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | number | 敌人索引（用于 `play_card` 的 `target_index`） |
| `enemy_id` | string | 内部 ID |
| `name` | string | 显示名称 |
| `current_hp` / `max_hp` | number | 生命值 |
| `block` | number | 格挡值 |
| `is_alive` | boolean | 是否存活 |
| `is_hittable` | boolean | 是否可被攻击 |
| `powers[]` | object[] | Power 列表（同 player.powers） |
| `move_id` | string \| null | 下一招内部 ID |
| `intents[]` | object[] | 意图列表 |

#### `combat.enemies[].intents[]`

| 字段 | 类型 | 说明 |
|------|------|------|
| `intent_type` | string | `Attack`、`Buff`、`StatusCard` 等 |
| `damage` | number \| null | 单次伤害 |
| `hits` | number \| null | 攻击次数 |
| `total_damage` | number \| null | 总伤害 |
| `status_card_count` | number \| null | 塞入状态牌数量 |

#### `*.dynamic_values[]`（适用于手牌、牌组、选牌、奖励卡、商店卡）

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 变量名（`Damage`、`Block`、`CalculatedDamage`、`Repeat`） |
| `base_value` | number | 基础值 |
| `current_value` | number | 当前预览值（UI 显示值） |
| `enchanted_value` | number | 附魔/永久修正后的值 |
| `is_modified` | boolean | 是否相对基础值变化 |

### `run` 子结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `floor` | number | 当前楼层 |
| `current_hp` / `max_hp` | number | 生命值 |
| `gold` | number | 金币 |
| `max_energy` | number | 基础最大能量 |
| `deck[]` | object[] | 牌组（含 card_id, name, upgraded, card_type, rarity, energy_cost, star_cost） |
| `relics[]` | object[] | 遗物（含 relic_id, name, description, stack, is_melted） |
| `potions[]` | object[] | 药水槽（含 potion_id, occupied, usage, target_type, can_use, can_discard, requires_target） |

### `map` 子结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `current_node` | object \| null | 当前坐标 `{ row, col }` |
| `is_travel_enabled` | boolean | 是否允许移动 |
| `available_nodes[]` | object[] | 可前往节点（含 index, row, col, node_type, state） |
| `nodes[]` | object[] | 完整图结构（含 parents, children 用于路线规划） |
| `boss_node` / `second_boss_node` | object \| null | Boss 坐标 |

**node_type 值**：`Monster`、`Elite`、`Boss`、`Rest`、`Shop`、`Event`、`Treasure`

### `event` 子结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `event_id` | string | 事件 ID |
| `title` | string | 标题 |
| `is_finished` | boolean | 是否已完成 |
| `options[]` | object[] | 选项（含 index, title, description, is_locked, is_proceed） |

### `shop` 子结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `is_open` | boolean | 库存面板是否打开 |
| `cards[]` / `relics[]` / `potions[]` | object[] | 商品（含 index, name, price, available） |
| `card_removal` | object \| null | 删牌服务（含 price, available） |

### `reward` 子结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `pending_card_choice` | boolean | 是否在卡牌选择子界面 |
| `can_proceed` | boolean | 是否可继续 |
| `rewards[]` | object[] | 奖励列表（含 index, reward_type, description, claimable） |
| `card_options[]` | object[] | 卡牌候选（含 index, card_id, name） |
| `alternatives[]` | object[] | 替代按钮（如"跳过"） |

**reward_type 值**：`Gold`、`Card`、`Potion`、`Relic`、`RemoveCard`、`SpecialCard`、`LinkedRewardSet`

### `selection` 子结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `kind` | string | 类型：`deck_card_select`、`deck_upgrade_select`、`deck_transform_select`、`deck_enchant_select` |
| `prompt` | string | 提示文字 |
| `cards[]` | object[] | 可选卡牌列表 |

### `rest.options[]`

| 字段 | 类型 | 说明 |
|------|------|------|
| `option_id` | string | `HEAL`、`SMITH`、`MEND`、`LIFT`、`COOK`、`DIG`、`HATCH`、`CLONE` |
| `title` | string | 标题 |
| `description` | string | 描述 |
| `is_enabled` | boolean | 是否可用 |

## 9. 回归测试操作手册

游戏更新或 Bridge 代码改动后，按以下步骤验证。

### 环境验证

```bash
dotnet --list-sdks           # 确认 9.0.x
ilspycmd --version           # 确认 9.x
godot --version              # 确认 4.5.x（显式路径调用）
```

### Mod 加载验证

1. 确认 `mods/` 下有 `.dll` + `.pck` + `mod_id.json`
2. 确认 `settings.save` 中 `mod_settings.mods_enabled = true`
3. 启动游戏
4. 检查游戏日志中有 Mod 扫描、初始化、HTTP 启动记录
5. `curl http://127.0.0.1:8080/health` 返回 200

### 奖励流程回归

1. 进入奖励房间
2. `claim_reward` 领取金币
3. `claim_reward` 领取卡牌奖励 → 进入卡牌选择子界面
4. `choose_reward_card` 选一张，或 `skip_reward_cards` 跳过
5. 重新读 state
6. 如果 `reward.can_proceed = true`，调 `proceed` 或 `collect_rewards_and_proceed`

**预期**：
- `skip_reward_cards` 只关闭卡牌 overlay，底层卡牌奖励可能仍可领取
- `proceed` 在主奖励界面可用时离开房间

### 选牌变体回归

1. 触发删除/升级/变形/附魔分支
2. 确认 `screen = CARD_SELECTION`
3. 确认 `selection.kind` 值正确
4. 调 `select_deck_card`
5. 确认回到原来的房间/事件

**预期**：
- 不会卡在确认状态
- 单选完成后 `CARD_SELECTION` 画面消失

### 战斗动态数据回归

1. 进入有费用变动的战斗（如 Bullet Time 改变卡牌费用）
2. 出牌前后分别读 state
3. 检查 `energy_cost`、`star_cost`、`costs_x`、`star_costs_x`、`unplayable_reason`

**预期**：
- 费用值反映战斗中的实时修正
- X 费标记是语义性的，不是从当前值复制的

### 商店流程回归

1. `open_shop_inventory`
2. `buy_card` / `buy_relic` / `buy_potion` / `remove_card_at_shop`
3. `close_shop_inventory`
4. `proceed` 离开商店

**预期**：
- `shop.is_open` 正确反映库存面板状态
- 购买后 `available` 字段更新
- 金币不足时购买返回 `invalid_action`

### 补丁后完整回归

每次游戏更新后：

1. 反编译新版本 sts2.dll，对比差异
2. 更新 Bridge 代码中受影响的部分
3. 重新构建 + 安装 Mod
4. 依次验证上述所有回归场景
5. 更新本文档中的版本基线

## 10. 逆向工程工具与方法

当游戏更新或首次分析时，需要反编译 `sts2.dll` 来了解游戏内部结构。

### 必要工具

| 工具 | 用途 | 安装方式 |
|------|------|---------|
| **ilspycmd** | 命令行反编译器，批量反编译到文件 | `dotnet tool install -g ilspycmd` |
| **ILSpy** | GUI 反编译器，浏览类型树和搜索 | [GitHub Releases](https://github.com/icsharpcode/ILSpy/releases)（Windows） |
| **AvaloniaILSpy** | ILSpy 的跨平台版本 | [GitHub](https://github.com/icsharpcode/AvaloniaILSpy)（macOS/Linux） |
| **dnSpyEx** | 带调试功能的反编译器，可附加到游戏进程 | [GitHub](https://github.com/dnSpyEx/dnSpy/releases)（仅 Windows） |
| **.NET 9 SDK** | 编译 Mod | `dotnet --list-sdks` 确认 9.x 已安装 |
| **Godot 4.5.1 Mono** | 打包 .pck 资源 | [Godot 官网](https://godotengine.org/download) |

**推荐组合**：

| 场景 | 工具 |
|------|------|
| 批量反编译到文件（CI/脚本化） | `ilspycmd` |
| 日常浏览和搜索类型/方法 | ILSpy GUI 或 AvaloniaILSpy |
| 调试游戏进程（排查运行时问题） | dnSpyEx（仅 Windows） |
| 对比版本差异 | `ilspycmd` 反编译两个版本 + `diff -r` |

### 工具安装验证

```bash
ilspycmd --version     # 应返回 9.x
dotnet --list-sdks     # 应包含 9.0.x
godot --version        # 应返回 4.5.1（通过显式路径调用）
```

### 步骤一：字符串扫描（快速探测）

不需要完整反编译，先扫描 sts2.dll 中的符号名，确认关键类型是否存在：

```bash
# 用 strings 或专用脚本扫描
strings "<游戏数据目录>/sts2.dll" | grep -i "CombatManager\|RunManager\|ModInitializer"
```

只能确认"有这个名字"，不知道具体用法。

### 步骤二：正式反编译

```bash
# 反编译整个 sts2.dll 到目录，产出完整 C# 项目结构
ilspycmd -p -o "./extraction/decompiled" "<游戏数据目录>/sts2.dll"

# 反编译后可以用任何编辑器打开，搜索和阅读 C# 源码
```

### 步骤三：重点搜索

在反编译结果中优先搜索以下关键词：

| 关键词 | 找什么 |
|--------|--------|
| `ModInitializer` | Mod 入口机制 |
| `RunManager` | 运行管理、动作队列 |
| `CombatManager` | 战斗管理、回合控制 |
| `ActionQueue` / `ActionQueueSynchronizer` | 动作入队和执行 |
| `CardModel` | 卡牌模型、出牌入口 |
| `Intent` | 敌人意图类型 |
| `Reward` | 奖励系统 |
| `Shop` / `Merchant` | 商店 |
| `Map` / `MapCoord` | 地图导航 |
| `Screen` / `ScreenContext` | 画面判定 |

### 步骤四：版本差异对比

游戏更新后，反编译新版本并对比：

```bash
# 旧版本已反编译在 extraction/v0.98/
# 反编译新版本
ilspycmd -p -o "./extraction/v0.99" "<新数据目录>/sts2.dll"

# 对比差异
diff -r ./extraction/v0.98 ./extraction/v0.99

# 重点关注：
# - 类名/方法名是否改了
# - 方法参数是否变了
# - 新增了什么类型
```

### 步骤五：验证

1. 编译最小 Mod → 安装到 `mods/` → 启动游戏
2. 检查 `/health` 是否返回 200
3. 逐步验证 `/state` 各字段
4. 逐步验证 `/action` 各动作
5. 回归测试已有功能

## 11. 常见问题

### Steam 启动失败：No appID found

直接运行 EXE 时 Steam API 拿不到 AppID。

**解决**：在游戏根目录创建 `steam_appid.txt`，内容写 `2868840`。或从 Steam 客户端启动游戏。

### Mod 加载被跳过

日志出现 `Skipping loading mod ... user has not yet seen the mods warning`。

**解决**：在 `%APPDATA%/SlayTheSpire2/default/1/settings.save` 中设置：
```json
{ "mod_settings": { "mods_enabled": true } }
```

### Mod DLL 找不到

**检查**：.dll 和 .pck 必须同目录、同名放在 `mods/` 下。`mod_id.json` 中的 `has_dll: true` 和 `has_pck: true` 必须正确。

## 12. 机制覆盖矩阵（已验证）

| 领域 | 覆盖度 | 说明 |
|------|--------|------|
| 主菜单流程 | 高 | continue_run、timeline gate、modal 已验证 |
| 奖励流程 | 高 | 领取、选牌、跳过、离开已验证 |
| 牌库选择 | 高 | 删除、升级、变形、附魔 4 种已验证 |
| 商店 | 高 | 购买卡/遗物/药水、删卡已验证 |
| 休息点 | 高 | 恢复、升级已验证 |
| 宝箱 | 高 | 开箱、选遗物、离开已验证 |
| 事件 | 高 | 普通选项、完成跳转、嵌套战斗已验证 |
| 药水 | 中 | 核心使用/丢弃已验证，边界情况需更多覆盖 |
| 动态费用 | 中 | Bullet Time 已验证，其他场景需扩展 |
| 角色广度 | 低 | 目前只深度测试了 Regent |
| 补丁后兼容 | 未知 | 每次游戏更新后需重新验证 |

### 最高价值待验证项

1. **角色广度**：至少测试一个非 Regent 角色
2. **生成卡牌**：战斗中生成的临时牌、复制牌、变形牌的状态表示
3. **不可打出原因**：触发更多 reason code 并确认字段稳定
4. **药水边界**：目标药水、队列药水、药水槽满时的替换行为
5. **遗物变异**：遗物修改费用、奖励、房间选项时的交互
