# C# Bridge 层详细设计

## 概述

Bridge 层是一个 C# Mod，运行在杀戮尖塔 2 的游戏进程内部。它通过直接访问游戏内部对象（CombatManager、RunManager 等），将游戏状态暴露为 HTTP JSON API，供外部 Agent 调用。

程序集名：`Spire2Mind.Bridge`
目标框架：.NET 9.0
依赖：sts2.dll、GodotSharp.dll、0Harmony.dll

## 目录结构

```
bridge/
├── Spire2Mind.Bridge.csproj
├── Spire2Mind.Bridge.sln
├── mod_id.json
├── Entry.cs                    ← [ModInitializer] 入口
├── server/
│   ├── HttpServer.cs           ← HTTP 监听 + 端口重试
│   ├── Router.cs               ← 5 端点路由 + 响应信封
│   ├── EventService.cs         ← SSE 事件轮询 + Channel 推送
│   └── JsonHelper.cs           ← JSON 序列化配置
├── game/
│   ├── ThreadContext.cs         ← 自定义 SynchronizationContext
│   ├── GameThread.cs           ← InvokeAsync<T> 对外 API
│   ├── StateBuilder.cs         ← 状态快照 + CanXxx 谓词
│   ├── ActionExecutor.cs       ← 动作执行 + WaitFor 稳定
│   └── Formatter.cs            ← Markdown 格式化
├── patches/
│   └── InstantMode.cs          ← Harmony 跳过动画
└── scripts/
    ├── build.sh
    └── build.ps1
```

## 1. 线程模型

### 问题

Godot 引擎要求所有游戏对象的读写在主线程上进行。HTTP 请求在线程池线程上到达。需要一种机制将工作编排到游戏主线程。

### 方案：自定义 SynchronizationContext + ConcurrentQueue

综合 STS2-Agent（API 优雅）和 STS2MCP（透明可控）两种方案的优点。

#### ThreadContext.cs

```csharp
internal sealed class GameThreadContext : SynchronizationContext
{
    private readonly ConcurrentQueue<(SendOrPostCallback, object?)> _queue = new();
    private readonly int _gameThreadId;

    public GameThreadContext()
    {
        _gameThreadId = Environment.CurrentManagedThreadId;
    }

    public override void Post(SendOrPostCallback callback, object? state)
        => _queue.Enqueue((callback, state));

    public void ProcessFrame()
    {
        var sw = Stopwatch.StartNew();
        while (_queue.TryDequeue(out var item))
        {
            item.Item1(item.Item2);
            if (sw.ElapsedMilliseconds > 8) break;
        }
    }

    public int PendingCount => _queue.Count;
    public bool IsGameThread => Environment.CurrentManagedThreadId == _gameThreadId;
}
```

#### 设计决策理由

**为什么不直接用 Godot 的 SynchronizationContext（STS2-Agent 方案）**：

- Godot 的 SyncContext 是黑箱——Post() 内部的调度时机、每帧处理量、异常传播行为都不可见
- 无帧保护——一帧内 Post 100 个任务可能全部执行，导致掉帧
- 依赖 Godot 的内部实现，版本升级可能改变行为

**为什么不直接用 ConcurrentQueue（STS2MCP 方案）**：

- 每个请求处理器都要手写 TaskCompletionSource 样板代码
- 没有 SynchronizationContext，async/await 链中的 continuation 可能跑到线程池线程上，违反主线程规则

**组合方案的优势**：

- **API 优雅**：继承 SynchronizationContext，InvokeAsync<T> 和 STS2-Agent 一样干净
- **透明可控**：Post() 内部走自己的 ConcurrentQueue，调度逻辑完全自己写
- **帧保护**：ProcessFrame() 按 8ms 时间预算限制每帧处理量（60fps 下一帧约 16ms，留一半给游戏）
- **可观测**：PendingCount 暴露队列堆积状况
- **Godot 版本无关**：只依赖 ProcessFrame 信号（最基础的 API）

#### GameThread.cs

```csharp
internal static class GameThread
{
    private static GameThreadContext _context = null!;

    public static void Initialize()
    {
        _context = new GameThreadContext();
        SynchronizationContext.SetSynchronizationContext(_context);
        NGame.Instance.GetTree().ProcessFrame += _context.ProcessFrame;
    }

    public static Task<T> InvokeAsync<T>(Func<T> action)
    {
        if (_context.IsGameThread)
            return Task.FromResult(action());

        var tcs = new TaskCompletionSource<T>(TaskCreationOptions.RunContinuationsAsynchronously);
        _context.Post(_ =>
        {
            try   { tcs.TrySetResult(action()); }
            catch (Exception ex) { tcs.TrySetException(ex); }
        }, null);
        return tcs.Task;
    }

    // 异步重载：用于 WaitFor* 方法中的 await ToSignal
    public static Task<T> InvokeAsync<T>(Func<Task<T>> action)
    {
        if (_context.IsGameThread)
            return action();

        var tcs = new TaskCompletionSource<T>(TaskCreationOptions.RunContinuationsAsynchronously);
        _context.Post(_ => _ = InvokeAsyncCore(action, tcs), null);
        return tcs.Task;
    }
}
```

**为什么需要异步重载**：ActionExecutor 内部用 `await ToSignal(ProcessFrame)` 做 WaitFor 等待。这个 await 的 continuation 需要回到游戏主线程。异步重载确保整条 async 链都在主线程上执行。

## 2. HTTP Server

### 端口重试启动

来自 STS2-Agent。游戏重启时老进程可能还没释放端口（Windows 上 error code 183），最多重试 20 次，每次间隔 250ms。

```csharp
private static HttpListener StartListenerWithRetry(string prefix)
{
    for (var attempt = 1; ; attempt++)
    {
        var listener = new HttpListener();
        listener.Prefixes.Add(prefix);
        try
        {
            listener.Start();
            return listener;
        }
        catch (HttpListenerException ex) when (IsPrefixConflict(ex) && attempt < 20)
        {
            listener.Close();
            Thread.Sleep(250);
        }
    }
}
```

端口通过环境变量 `STS2_API_PORT` 配置，默认 8080。

### 请求分发

每个请求 fire-and-forget 到线程池，互不阻塞：

```csharp
context = await listener.GetContextAsync();
_ = Task.Run(() => Router.HandleAsync(context, cancellationToken));
```

## 3. Router：5 端点

| 方法 | 路径 | 功能 | 线程编排 |
|------|------|------|---------|
| GET | /health | 版本信息 | 无需编排 |
| GET | /state | 完整游戏状态 | GameThread.InvokeAsync |
| GET | /state?format=markdown | Markdown 格式状态 | GameThread.InvokeAsync |
| GET | /actions/available | 可用动作列表 | GameThread.InvokeAsync |
| GET | /events/stream | SSE 事件流 | Channel 订阅 |
| POST | /action | 执行游戏动作 | GameThread.InvokeAsync |

### 响应信封（来自 STS2-Agent）

统一格式，每个请求带 request_id 便于排查：

```json
{ "ok": true,  "request_id": "req_...", "data": { ... } }
{ "ok": false, "request_id": "req_...", "error": { "code": "...", "message": "...", "retryable": false } }
```

### Markdown 双格式（来自 STS2MCP）

支持 `?format=markdown` 查询参数。Markdown 对 LLM 更友好，token 消耗更少。

## 4. SSE 事件流

### EventService

来自 STS2-Agent 的完整设计。后台线程每 120ms 轮询游戏状态，通过 StateDigest 轻量摘要对比检测变化，发现变化就推送事件。

#### 事件类型

| 事件 | 触发条件 |
|------|---------|
| session_started | 首次检测到游戏状态 |
| screen_changed | 画面切换（COMBAT→REWARD 等） |
| combat_started / combat_ended | 进入/离开战斗 |
| combat_turn_changed | 回合数变化 |
| player_action_window_opened / closed | 轮到/不轮到玩家操作 |
| route_decision_required | 需要选地图路径 |
| reward_decision_required | 需要选奖励 |
| event_state_changed | 事件选项变化 |
| available_actions_changed | 可用动作列表变化 |

#### 订阅模型

- 每个订阅者一个 Bounded Channel（容量 256）
- DropOldest 策略：慢消费者不阻塞，丢弃最旧事件
- 新订阅者立即收到 stream_ready 快照

#### SSE 端点

- 15 秒心跳保持连接活跃
- 有事件时批量读出所有待发事件
- 优雅处理客户端断开

## 5. StateBuilder

### 状态快照

Fork STS2-Agent 的 GameStateService（约 5600 行），包含：

- 19 个可空状态分区（combat, map, shop, event, rest, reward, chest, selection, modal 等）
- 40+ 个 CanXxx() 可用性谓词，汇总为 available_actions 数组
- agent_view 精简视图（给 AI 减少 token）

### 关键改进

**抽牌堆打乱（来自 STS2MCP）**：

```csharp
var drawPile = player.DrawPile.Cards.ToList();
drawPile.Shuffle();  // 不泄露抽牌顺序，保证公平性
```

**Modal 优先级**：

弹窗时只返回 confirm_modal / dismiss_modal，屏蔽其他动作。

## 6. ActionExecutor

### 统一执行模式

Fork STS2-Agent 的 GameActionService（约 4000 行），每个动作遵循 5 步模式：

1. **验证**：CanXxx() 检查
2. **参数检查**：index 范围验证
3. **执行**：RequestEnqueue / TryManualPlay / ForceClick
4. **等待稳定**：WaitForTransition（yield 到 Godot 帧信号）
5. **返回**：ActionResponse + 最新状态快照

### WaitFor 机制

不是 Thread.Sleep，而是用 Godot 的帧信号做协作式等待：

```csharp
private static async Task<bool> WaitForTransition(Func<bool> isStable, TimeSpan timeout)
{
    var deadline = DateTime.UtcNow + timeout;
    while (DateTime.UtcNow < deadline)
    {
        await NGame.Instance.ToSignal(
            NGame.Instance.GetTree(), SceneTree.SignalName.ProcessFrame);
        if (isStable()) return true;
    }
    return isStable();
}
```

每个动作有不同的"稳定"定义（回合推进、卡牌离开手牌、画面切换等）。

### 错误分级

| 状态码 | 错误码 | 含义 | retryable |
|--------|--------|------|-----------|
| 400 | invalid_request | 缺少参数 | false |
| 409 | invalid_action | 当前状态不允许 | false |
| 409 | invalid_target | 目标越界 | false |
| 503 | state_unavailable | 游戏对象暂时不可用 | true |

## 7. Harmony 补丁

### InstantMode（来自 STS2MCP）

通过 Harmony 运行时补丁往游戏设置界面注入第三个速度选项（Normal / Fast / Instant），跳过动画加速 AI 对局。

放在 `patches/` 独立目录，与核心逻辑分离——补丁挂了不影响基本功能。

## 8. 构建与安装

```bash
# 构建
cd bridge/
dotnet build -c Release -p:Sts2DataDir="${STS2_DATA_DIR}"

# 安装到游戏
cp bin/Release/net9.0/Spire2Mind.Bridge.dll "${STS2_GAME_DIR}/mods/"
cp mod_id.json "${STS2_GAME_DIR}/mods/"
```

输出文件：
- `Spire2Mind.Bridge.dll` — Mod 逻辑
- `Spire2Mind.Bridge.pck` — Godot 资源包
- `mod_id.json` — Mod 清单
