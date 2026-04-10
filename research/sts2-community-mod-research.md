# STS2 社区 Mod 生态深度调研

> 调研日期: 2026-04-10
> 调研目标: 从社区 mod 源码中提取可用于 Bridge 重构的模式、API、架构

---

## 1. 游戏核心 API 速查

### 1.1 关键命名空间

| 命名空间 | 用途 |
|---------|------|
| `MegaCrit.Sts2.Core.Modding` | ModInitializer, ModHelper, ModManager |
| `MegaCrit.Sts2.Core.Models` | ModelDb, AbstractModel, SingletonModel |
| `MegaCrit.Sts2.Core.Runs` | RunManager, RunState, IRunState |
| `MegaCrit.Sts2.Core.Combat` | CombatManager, CombatState, CombatSide |
| `MegaCrit.Sts2.Core.Context` | LocalContext, PlayerChoiceContext |
| `MegaCrit.Sts2.Core.Entities.Players` | Player, PlayerCombatState |
| `MegaCrit.Sts2.Core.Entities.Creatures` | Creature, power/effect hooks |
| `MegaCrit.Sts2.Core.Entities.Cards` | CardModel, CardPile, PileType, CardType, CardRarity, TargetType |
| `MegaCrit.Sts2.Core.Entities.Relics` | RelicModel, RelicStatus, RelicRarity |
| `MegaCrit.Sts2.Core.Entities.Powers` | PowerModel, PowerType, PowerStackType |
| `MegaCrit.Sts2.Core.Commands` | PowerCmd, CardCmd, CreatureCmd, PlayerCmd, CardPileCmd, SfxCmd |
| `MegaCrit.Sts2.Core.Saves` | SaveManager, SerializableRun |
| `MegaCrit.Sts2.Core.Nodes` | NGame, NRun 及各种 UI 节点 |
| `MegaCrit.Sts2.Core.Nodes.Combat` | NHealthBar, NPower, NHandCardHolder |
| `MegaCrit.Sts2.Core.Nodes.Screens.PauseMenu` | NPauseMenu, NPauseMenuButton |
| `MegaCrit.Sts2.Core.MonsterMoves.Intents` | AttackIntent, IntentType |
| `MegaCrit.Sts2.Core.HoverTips` | HoverTipFactory, IHoverTip |
| `MegaCrit.Sts2.Core.GameActions.Multiplayer` | PlayerChoiceContext, CardPlay |
| `MegaCrit.Sts2.Core.Multiplayer` | NetGameType, NetSingleplayerGameService |
| `Godot.Bridge` | ScriptManagerBridge |

### 1.2 核心单例访问

```csharp
// 玩家
Player player = LocalContext.GetMe(RunManager.Instance?.State);

// 战斗
CombatManager.Instance.IsInProgress       // 是否在战斗中
CombatManager.Instance.IsEnemyTurnStarted // 是否敌方回合
CombatState combatState = player.Creature.CombatState;

// 牌堆
PileType.Hand.GetPile(player).Cards       // 手牌
PileType.Draw.GetPile(player).Cards       // 抽牌堆
PileType.Discard.GetPile(player).Cards    // 弃牌堆
PileType.Exhaust.GetPile(player).Cards    // 排除堆

// Power
creature.GetPower<VulnerablePower>()?.Amount  // 查询任意 power 层数

// 遗物
player.GetRelic<SomeRelic>()?.Status      // 查询遗物状态
player.Relics                              // 所有遗物列表

// 敌人
combatState.HittableEnemies               // 可攻击的敌人列表
hittableEnemy.Monster.NextMove.Intents    // 敌人意图
intent.IntentType                          // Attack, DeathBlow, etc.
AttackIntent.GetTotalDamage(targets, owner) // 意图伤害计算

// 战斗历史
CombatManager.Instance.History.CardPlaysFinished // 已打出的卡牌

// 模型数据库
ModelDb.AllCards                            // 所有卡牌
ModelDb.AllRelics                           // 所有遗物
ModelDb.AllPowers                           // 所有 power
ModelDb.AllPotions                          // 所有药水
ModelDb.AllCharacters                       // 所有角色
ModelDb.Card<T>()                           // 按类型获取卡牌模型
ModelDb.Relic<T>()                          // 按类型获取遗物模型
ModelDb.Power<T>()                          // 按类型获取 power 模型

// 存档
SaveManager.Instance.HasRunSave
SaveManager.Instance.LoadRunSave()         // → ReadSaveResult<SerializableRun>
RunState.FromSerializable(serializableRun) // 反序列化
```

### 1.3 游戏流程控制

```csharp
// 快速重启完整流程 (from StS2-Quick-Restart)
RunManager.Instance.ActionQueueSet.Reset();              // 清空动作队列
NRunMusicController.Instance.StopMusic();                // 停止音乐
RunManager.Instance.CleanUp();                           // 完整清理
var save = SaveManager.Instance.LoadRunSave();            // 加载存档
var runState = RunState.FromSerializable(save.Data);      // 反序列化
RunManager.Instance.SetUpSavedSinglePlayer(runState, save.Data); // 重建状态
SfxCmd.Play(runState.Players[0].Character.CharacterTransitionSfx);
NGame.Instance.ReactionContainer.InitializeNetworking(new NetSingleplayerGameService());
TaskHelper.RunSafely(NGame.Instance.LoadRun(runState, save.Data.PreFinishedRoom));

// 网络类型检查
RunManager.Instance.NetService.Type == NetGameType.Singleplayer
```

---

## 2. BaseLib API 完整速查

### 2.1 CustomSingletonModel（被动 Hook 系统）

来源: `BaseLib/Abstracts/CustomSingletonModel.cs`

```csharp
public abstract class CustomSingletonModel : SingletonModel, ICustomModel
{
    // 构造：指定是否接收 combat 和 run hooks
    public CustomSingletonModel(bool receiveCombatHooks, bool receiveRunHooks)
}
```

**注册方式**: 内部通过反射调用 `ModHelper.SubscribeForCombatStateHooks()` 和 `ModHelper.SubscribeForRunStateHooks()`。

**可用的 Combat Hooks**（继承自 `SingletonModel`，可 override）:

| Hook | 参数 | 触发时机 |
|------|------|---------|
| `AfterCurrentHpChanged` | `Creature, decimal delta` | HP 变化后 |
| `AfterBlockGained` | `Creature, decimal, ValueProp, CardModel?` | 获得格挡后 |
| `AfterCardDrawn` | `PlayerChoiceContext, CardModel, bool` | 抽牌后 |
| `AfterCardPlayed` | `PlayerChoiceContext, CardPlay` | 打牌后 |
| `AfterCardPlayedLate` | `PlayerChoiceContext, CardPlay` | 打牌后（晚期） |
| `AfterEnergySpent` | `CardModel, int` | 消耗能量后 |
| `AfterSideTurnStart` | `CombatSide, CombatState` | 回合开始（任意方） |
| `AfterPlayerTurnStart` | `PlayerChoiceContext, Player` | 玩家回合开始 |
| `AfterTurnEnd` | `PlayerChoiceContext, CombatSide` | 回合结束 |
| `AfterCombatEnd` | `CombatRoom` | 战斗结束 |
| `AfterStarsSpent` | `int, Player` | 消耗星星后 |
| `AfterAttack` | `AttackCommand` | 攻击后 |
| `BeforeHandDraw` | `Player, PlayerChoiceContext, CombatState` | 抽牌前 |
| `BeforeSideTurnStart` | `PlayerChoiceContext, CombatSide` | 回合开始前 |
| `AfterEnergyReset` | `Player` | 能量重置后 |

**Bridge 用法**:
```csharp
public class BridgeHooker : CustomSingletonModel
{
    public BridgeHooker() : base(receiveCombatHooks: true, receiveRunHooks: true) { }
    
    public override async Task AfterCardPlayed(PlayerChoiceContext ctx, CardPlay play)
    {
        // 零延迟推送事件到 EventService
    }
    // ...其他 hook
}
```

### 2.2 SpireField（弱引用状态附加）

来源: `BaseLib/Utils/SpireField.cs`

```csharp
// 基本用法：在任意对象上挂载数据
public class SpireField<TKey, TVal> where TKey : class
{
    public SpireField(Func<TVal?> defaultVal)
    public SpireField(Func<TKey, TVal?> defaultVal)  // 按实例生成默认值
    
    public TVal? Get(TKey obj)
    public void Set(TKey obj, TVal? val)
    public TVal? this[TKey obj] { get; set; }        // 索引器语法
}

// 可持久化版本（会写入存档）
public class SavedSpireField<TKey, TVal> : SpireField<TKey, TVal>, ISavedSpireField
    where TKey : class
{
    public SavedSpireField(Func<TVal?> defaultVal, string name)
}
```

**Bridge 用法**: 可以在 CardModel/Creature 上标记已处理状态，解决 index drift 问题：
```csharp
static SpireField<CardModel, int> BridgeActionEpoch = new(() => 0);
// 执行前标记：BridgeActionEpoch[card] = currentEpoch;
// 验证时检查：BridgeActionEpoch[card] == currentEpoch
```

### 2.3 ModConfig 系统

来源: `BaseLib/Config/`

```csharp
// 声明配置
public class BridgeConfig : SimpleModConfig
{
    [ConfigSection("server")]
    public static int ApiPort { get; set; } = 8080;
    
    [SliderRange(50, 500, 50)]
    [SliderLabelFormat("{0}ms")]
    public static int PollIntervalMs { get; set; } = 120;
    
    [ConfigSection("debug")]
    public static bool VerboseLogging { get; set; } = false;
    
    [ConfigHideInUI]  // 存档但不显示 UI
    public static string LastRunId { get; set; } = "";
}

// 注册
ModConfigRegistry.Register(ModId, new BridgeConfig());

// 使用
var port = BridgeConfig.ApiPort;
```

属性标签: `[ConfigSection]`, `[SliderRange]`, `[SliderLabelFormat]`, `[ConfigHoverTip]`, `[ConfigIgnore]`, `[ConfigHideInUI]`, `[ConfigTextInput]`, `[ConfigButton]`, `[ConfigVisibleWhen]`

### 2.4 ModInterop（跨 Mod 通信）

来源: `BaseLib/Patches/Features/ModInteropPatch.cs`

```csharp
// 声明对另一个 mod 的软依赖
[ModInterop("OtherModId")]
public class OtherModBridge : InteropClassWrapper
{
    [InteropTarget("TargetType", "MethodName")]
    public static void CallOtherModMethod(object instance, string arg) { }
}
```

工作原理: 在 post-mod-init 阶段检查目标 mod 是否加载，如果加载则通过 Harmony transpiler 重写方法体，注入实际调用。

### 2.5 CommonActions（战斗工具函数）

来源: `BaseLib/Utils/CommonActions.cs`

```csharp
// 攻击
CommonActions.CardAttack(card, cardPlay, hitCount: 1).WithHitFx("vfx/...").Execute(ctx)
CommonActions.CardAttack(card, target, damage, hitCount).Execute(ctx)

// 格挡
await CommonActions.CardBlock(card, cardPlay)

// 抽牌
await CommonActions.Draw(card, ctx)

// 施加 Power
await CommonActions.Apply<VulnerablePower>(target, card)
await CommonActions.Apply<StrengthPower>(targets, card)  // 多目标
await CommonActions.ApplySelf<StrengthPower>(card)        // 对自己

// 选牌
await CommonActions.SelectCards(card, prompt, ctx, PileType.Hand, count)
await CommonActions.SelectSingleCard(card, prompt, ctx, PileType.Discard)
```

### 2.6 内容注册机制

所有自定义模型通过构造函数自动注册:
- `CustomCardModel` → `CustomContentDictionary.AddModel()`
- `CustomRelicModel` → 同上
- `CustomPowerModel` → 同上
- `CustomPotionModel` → 同上
- `CustomEventModel` → `CustomContentDictionary.AddEvent()`
- `CustomEncounterModel` → `CustomContentDictionary.AddEncounter()`
- `CustomOrbModel` → 静态 `RegisteredOrbs` 列表
- `CustomCharacterModel` → `ModelDbCustomCharacters.Register()`
- `CustomAncientModel` → `CustomContentDictionary.AddAncient()`

ID 生成规则: 根命名空间前缀 + "-" + 类名小写，可用 `[CustomID("xxx")]` 覆盖。

Pool 归属: 用 `[Pool(typeof(SomeCardPool))]` 属性标记。

---

## 3. Harmony Patch 模式大全

### 3.1 基本模式

```csharp
// Postfix: 在原方法之后执行
[HarmonyPatch(typeof(NPauseMenu), "_Ready")]
[HarmonyPostfix]
static void AfterReady(NPauseMenu __instance) { }

// Prefix: 在原方法之前执行，return false 可跳过原方法
[HarmonyPatch(typeof(SomeClass), "SomeMethod")]
[HarmonyPrefix]
static bool BeforeMethod(SomeClass __instance, ref string __result) { return true; }

// Transpiler: IL 级代码修改
[HarmonyPatch(typeof(SomeClass), "SomeMethod")]
[HarmonyTranspiler]
static IEnumerable<CodeInstruction> Transpile(IEnumerable<CodeInstruction> instructions)
{
    return new CodeMatcher(instructions)
        .MatchForward(false, new CodeMatch(OpCodes.Call, someMethod))
        .InsertAndAdvance(new CodeInstruction(OpCodes.Call, myMethod))
        .InstructionEnumeration();
}
```

### 3.2 私有字段访问（Publicizer 模式）

```csharp
// .csproj 配置
<PackageReference Include="BepInEx.AssemblyPublicizer.MSBuild" Version="0.4.3" />
// 或 Krafs.Publicizer
<PackageReference Include="Krafs.Publicizer" Version="2.3.0" />

<Reference Include="sts2">
    <Publicize>True</Publicize>
</Reference>

// Harmony patch 中直接访问私有字段（4 个下划线前缀）
static void Postfix(Control ____buttonContainer, NPauseMenuButton ____settingsButton) { }

// 或者通过 publicizer 直接在代码中访问
var privateField = instance._somePrivateField;  // 编译期可见
```

### 3.3 异步方法补丁

```csharp
// 对 async 方法的补丁
[HarmonyPatch(typeof(SomeClass), "SomeAsyncMethod", MethodType.Async)]
static async Task Postfix(Task __result) { await __result; /* 后续逻辑 */ }

// 通过 AsyncStateMachineAttribute 找到状态机 MoveNext
var attr = method.GetCustomAttribute<AsyncStateMachineAttribute>();
var moveNext = attr.StateMachineType.GetMethod("MoveNext", ...);
```

### 3.4 属性 getter 补丁

```csharp
[HarmonyPatch(typeof(RelicModel), nameof(RelicModel.PackedIconPath), MethodType.Getter)]
static bool Prefix(RelicModel __instance, ref string __result) { ... }
```

### 3.5 执行顺序控制

```csharp
[HarmonyAfter("SomeOtherModId")]  // 在某个 mod 的 patch 之后执行
[HarmonyBefore("AnotherModId")]   // 在某个 mod 的 patch 之前执行
```

---

## 4. UI 注入模式

### 4.1 创建 Label

```csharp
var label = new Label
{
    Name = "MyLabel",
    Text = "←42",
    HorizontalAlignment = HorizontalAlignment.Center,
    VerticalAlignment = VerticalAlignment.Center,
};
label.AddThemeFontOverride("font", GD.Load<Font>("res://fonts/kreon_bold.ttf"));
label.AddThemeFontSizeOverride("font_size", 24);
label.AddThemeColorOverride("font_color", Colors.Salmon);
parentNode.AddChild(label);
```

### 4.2 克隆按钮

```csharp
var newButton = (NPauseMenuButton)existingButton.Duplicate();
newButton.Name = "MyButton";
container.AddChild(newButton);
container.MoveChild(newButton, desiredIndex);
newButton.Connect(NClickableControl.SignalName.Released, 
    Callable.From<NButton>(_ => OnPressed()));
```

### 4.3 WeakNodeRegistry（安全节点跟踪）

```csharp
// 注册
WeakNodeRegistry<Label>.Register(myLabel);

// 批量更新（自动清理已释放节点）
WeakNodeRegistry<Label>.ForEachLive(label => label.Text = "updated");

// 按 index 获取
if (WeakNodeRegistry<Label>.TryGetLiveNode(index, out var label)) { ... }
```

### 4.4 Tween 动画

```csharp
var tween = CreateTween();
tween.TweenProperty(node, "modulate:a", 1.0f, 0.2f);  // 0.2 秒淡入
tween.TweenCallback(Callable.From(OnComplete));
```

---

## 5. 构建系统配置模板

### 5.1 最小 .csproj（带 Publicizer + BaseLib）

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net9.0</TargetFramework>
    <Nullable>enable</Nullable>
    <AllowUnsafeBlocks>true</AllowUnsafeBlocks>
  </PropertyGroup>

  <ItemGroup>
    <Reference Include="sts2">
      <HintPath>$(Sts2DataDir)/sts2.dll</HintPath>
      <Private>false</Private>
      <Publicize>True</Publicize>
    </Reference>
    <Reference Include="GodotSharp">
      <HintPath>$(Sts2DataDir)/GodotSharp.dll</HintPath>
      <Private>false</Private>
    </Reference>
    <Reference Include="0Harmony">
      <HintPath>$(Sts2DataDir)/0Harmony.dll</HintPath>
      <Private>false</Private>
    </Reference>
  </ItemGroup>

  <ItemGroup>
    <PackageReference Include="BepInEx.AssemblyPublicizer.MSBuild" Version="0.4.3" />
    <PackageReference Include="Alchyr.Sts2.BaseLib" Version="*" />
  </ItemGroup>
</Project>
```

### 5.2 Mod Manifest 模板

```json
{
  "id": "Spire2Mind.Bridge",
  "name": "Spire2Mind Bridge",
  "author": "spire2mind",
  "description": "Game state extraction and control bridge for AI automation.",
  "version": "0.2.0",
  "has_pck": true,
  "has_dll": true,
  "dependencies": ["BaseLib"],
  "affects_gameplay": false
}
```

---

## 6. Bridge 重构可直接借鉴的代码模式

### 6.1 替代 ReflectionUtils 的直接访问（需 Publicizer）

当前 Bridge 的反射调用 → Publicizer 后的直接调用对照:

| 当前 (反射) | 替换为 (直接) |
|------------|-------------|
| `ReflectionUtils.GetMemberValue(instance, "SomeField")` | `instance.SomeField` |
| `ReflectionUtils.InvokeMethod(instance, "SomeMethod", args)` | `instance.SomeMethod(args)` |
| `ReflectionUtils.GetMemberValue<bool>(instance, "IsEnabled")` | `instance.IsEnabled` |
| `intent.GetTotalDamage(targets, enemy)` via reflection | `((AttackIntent)intent).GetTotalDamage(targets, enemy)` |

### 6.2 EventService 混合模式架构

```
现在:  PeriodicTimer(120ms) → Build() → Diff(StateDigest) → Publish
改后:  CustomSingletonModel hooks → 立即 Publish(精确事件)
       + PeriodicTimer(500ms) → Build() → Diff(增强 Digest) → Publish(兜底)
```

精确事件包含参数（谁打了谁、打了多少伤害），轮询兜底捕获 hook 未覆盖的 UI 变化。

### 6.3 自定义 Hook Dispatch（from WatcherMod）

```csharp
// 游戏内部的 hook 迭代器
combatState.IterateHookListeners()  // → IEnumerable<AbstractModel>

// 注册
ModHelper.SubscribeForCombatStateHooks(modId, combatState => 
    new[] { myBridgeHooker });
```

### 6.4 TaskHelper.RunSafely（替代自定义调度）

```csharp
// 当前: GameThread.InvokeAsync(() => { ... })
// 可考虑: TaskHelper.RunSafely(async () => { ... })
// 注意: 需要验证是否支持从非游戏线程调用
```

---

## 7. 控制台命令列表（from freude916 文档）

| 命令 | 用途 |
|------|------|
| `gold [amount]` | 设置/增加金币 |
| `stars [amount]` | 设置/增加星星 |
| `heal [amount]` | 治疗 |
| `damage [amount]` | 受伤 |
| `energy [amount]` | 设置能量 |
| `block [amount]` | 设置格挡 |
| `card <ID> [pile]` | 添加卡牌到指定牌堆 |
| `relic [add/remove] <ID>` | 添加/移除遗物 |
| `potion <ID>` | 添加药水 |
| `fight <ID>` | 开始指定战斗 |
| `win` | 赢得当前战斗 |
| `die` | 死亡 |
| `kill` | 杀死所有敌人 |
| `act [number]` | 跳转到指定幕 |
| `room [type]` | 跳转到指定房间类型 |
| `event [ID]` | 触发指定事件 |
| `god` | 上帝模式（无敌） |
| `instant` | 跳过动画 |
| `dump` | 导出调试信息快照 |

---

## 8. 多人架构要点（from freude916 文档）

- 模式: Client-Host 模型
- `affects_gameplay` 的 mod 必须所有玩家一致
- Host 广播所有动作，Client 也会收到自己请求的回放
- Modded 模式使用独立存档目录（`modded/`）
- 启动参数: `--nomods`, `--autoslay`, `--seed <seed>`, `--fastmp`

---

## 9. 调研来源索引

| 项目 | 本地路径 | 关键文件 |
|------|---------|---------|
| BaseLib-StS2 | `research/BaseLib-StS2/` | `src/Abstracts/CustomSingletonModel.cs`, `src/Utils/SpireField.cs`, `src/Utils/CommonActions.cs`, `src/Config/` |
| Minty-Spire-2 | `research/Minty-Spire-2/` | `src/util/MintyHooker.cs`, `src/util/Wiz.cs`, `src/util/WeakNodeRegistry.cs`, `src/features/SummedIncomingDamageRender.cs` |
| StS2-Quick-Restart | `research/StS2-Quick-Restart/` | `QuickRestart.cs`（RestartRoom 完整流程）, `MainFile.cs`（UI 注入）, `.csproj`（Publicizer 配置） |
| WatcherMod | `research/WatcherMod/` | `src/WatcherHook.cs`（hook dispatch）, `src/WatcherModel.cs`（SpireField + stance）, `src/WatcherModelDb.cs`（ModelDb 查询） |
| freude916/sts2-quickRestart | `research/sts2-quickRestart/` | `README.md`（最全面的中文 modding 指南） |
| ModTemplate-StS2 | `research/ModTemplate-StS2/` | `wiki/`（setup、hooks、content 文档） |
