# spire2mind 整体架构设计

## 项目定位

spire2mind 是一个 AI 自动玩杀戮尖塔 2 (Slay the Spire 2) 的系统。它由三层组成：C# Bridge（游戏桥接）、Go Agent（AI 编排）、bubbletea TUI（终端界面）。

## 架构全景

```
┌─────────────────────────────────────────────────────┐
│                  终端 TUI (bubbletea)                │
│  游戏状态 | Agent 思考过程 | 操作日志 | Token/Cost    │
└───────────────┬─────────────────────────────────────┘
                │ tea.Msg 事件驱动
                ▼
┌─────────────────────────────────────────────────────┐
│              Go Agent (open-agent-sdk-go)            │
│                                                      │
│  Harness Loop:                                       │
│    callModel() → tool_use → RunTools() → loop        │
│                                                      │
│  Custom Tools:                                       │
│    get_game_state  → ModClient.GetState()            │
│    get_actions     → ModClient.GetActions()          │
│    act             → ModClient.Act()                 │
│    wait_actionable → ModClient.WaitUntilActionable() │
│    get_game_data   → 本地 JSON 查询                   │
└───────────────┬─────────────────────────────────────┘
                │ HTTP JSON (localhost:8080)
                ▼
┌─────────────────────────────────────────────────────┐
│              C# Bridge (游戏 Mod)                    │
│                                                      │
│  Server:  HttpServer + Router (5端点) + SSE          │
│  Thread:  自定义 SynchronizationContext + Queue       │
│  State:   StateBuilder (状态快照 + CanXxx 谓词)      │
│  Action:  ActionExecutor (执行 + WaitFor 稳定)       │
│  Patch:   Harmony InstantMode                        │
└───────────────┬─────────────────────────────────────┘
                │ 进程内直接调用
                ▼
┌─────────────────────────────────────────────────────┐
│              Slay the Spire 2 (Godot 引擎)           │
└─────────────────────────────────────────────────────┘
```

## 各层职责

### C# Bridge 层 — "手和眼"

运行在游戏进程内部，职责是**翻译**：把游戏内部对象翻译成 JSON，把 JSON 动作翻译成游戏操作。

- 不做决策
- 不存记忆
- 不知道 AI 的存在
- 尽可能薄，只做翻译

详细设计见 [bridge-design.md](bridge-design.md)。

### Go Agent 层 — "大脑"

独立进程，通过 HTTP 连接 Bridge，职责是**驱动 AI 玩游戏**。

- 使用 open-agent-sdk-go 的 harness 模式（LLM 驱动循环）
- 把游戏操作封装成 Custom Tools
- LLM 通过工具与游戏交互
- Agent SDK 自动管理：调 LLM → 执行工具 → 喂回结果 → 循环

详细设计见 [agent-design.md](agent-design.md)。

### bubbletea TUI — "界面"

终端界面，职责是**展示**：把 Agent 的运行状态实时渲染到终端。

- 基于 Elm Architecture（Init → Update → View）
- 通过 tea.Cmd 桥接 Agent SDK 的事件流
- 显示游戏状态、Agent 思考过程、操作日志、Token 消耗

## 数据流

### 正常操作流

```
Agent SDK 调 LLM
  → LLM 返回 tool_use: get_game_state
  → SDK 执行 GetStateTool.Call()
    → ModClient: GET http://localhost:8080/state
      → Router → GameThread.InvokeAsync(StateBuilder.Build)
        → 游戏主线程读 CombatManager / RunManager
      ← JSON 返回
  → ToolResult 喂回 LLM
  → LLM 返回 tool_use: act("play_card", card_index=2)
  → SDK 执行 ActTool.Call()
    → ModClient: POST http://localhost:8080/action
      → Router → GameThread.InvokeAsync(ActionExecutor.ExecuteAsync)
        → 验证 → 执行 → WaitFor 稳定 → 构建新状态
      ← JSON 返回
  → ToolResult 喂回 LLM
  → 循环继续...
```

### SSE 事件等待流

```
LLM 返回 tool_use: wait_until_actionable
  → SDK 执行 WaitUntilActionableTool.Call()
    → 第一级：GET /state → 可操作？→ 直接返回
    → 第二级：GET /events/stream → SSE 等待匹配事件
      ← Bridge 的 EventService 每 120ms 轮询状态变化
      ← 检测到 player_action_window_opened → 推送事件
    → 第三级：SSE 失败 → 每 250ms 轮询 GET /state
  → 返回最新状态
```

### TUI 更新流

```
Agent SDK eventCh ← SDKMessage
  → bubbletea listenAgent() tea.Cmd
    → AgentEventMsg 进入 Update()
      → 更新 model（gameState, logs, usage）
      → View() 重绘终端
      → return listenAgent() 继续监听
```

## 技术选型

| 组件 | 选择 | 理由 |
|------|------|------|
| 游戏 Mod | C# / .NET 9 | 游戏基于 Godot + .NET，Mod 必须是 C# |
| Agent 框架 | open-agent-sdk-go | Go 实现的 harness 模式，自带工具执行、流式输出、成本追踪 |
| TUI 框架 | bubbletea | Go 生态最成熟的 TUI 库，Elm Architecture |
| LLM | Claude (默认) | 通过 Agent SDK 配置，可切换 |
| 通信协议 | HTTP JSON + SSE | 简单可靠，请求-响应 + 事件推送 |

## 项目结构

```
spire2mind/
├── bridge/                     ← C# Mod
│   ├── Spire2Mind.Bridge.csproj
│   ├── Entry.cs
│   ├── server/
│   ├── game/
│   ├── patches/
│   └── scripts/
├── internal/                   ← Go 私有包
│   ├── tui/                    ← bubbletea TUI
│   └── game/                   ← 游戏工具 + 客户端
├── data/
│   └── eng/                    ← 游戏元数据 JSON
├── cmd/
│   └── spire2mind/
│       └── main.go             ← Go 入口
├── research/                   ← 调研项目引用
├── docs/                       ← 设计文档
├── go.mod
└── README.md
```

## 设计原则

1. **Bridge 尽可能薄** — 只做翻译，不做智能。越薄越稳定，越容易跟游戏版本同步
2. **Agent 层可编程** — 区别于 STS2-Agent 的"裸 LLM 调工具"模式，Go 代码控制流程
3. **关注点分离** — Bridge 不知道 AI，Agent 不知道游戏内部，TUI 不知道游戏逻辑
4. **降级容错** — SSE 不可用时降级到轮询，动作超时时返回 pending 状态
5. **公平性** — 抽牌堆打乱，不给 AI 超出人类玩家的信息优势

## 参考项目

- [STS2-Agent](../research/STS2-Agent) — 主要参考，Fork 其 Mod 层
- [STS2MCP](../research/STS2MCP) — 借鉴 Markdown 输出、InstantMode、抽牌堆打乱
- [open-agent-sdk-go](../research/open-agent-sdk-go) — Agent 框架
- [bubbletea](../research/bubbletea) — TUI 框架
