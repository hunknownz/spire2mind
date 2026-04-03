# Go Agent 层详细设计

## 概述

Agent 层是一个 Go 程序，使用 open-agent-sdk-go 的 harness 模式驱动 AI 玩杀戮尖塔 2。它通过 HTTP 连接 C# Bridge，通过 LLM API（Claude/GPT）进行决策，通过 bubbletea 渲染终端界面。

## 目录结构

```
internal/
├── tui/                    ← bubbletea TUI
│   ├── model.go            ← Model 定义 + Init
│   ├── update.go           ← Update 逻辑 + Agent 桥接
│   ├── view.go             ← View 渲染
│   └── styles.go           ← lipgloss 样式
└── game/                   ← 游戏工具 + 客户端
    ├── tools.go            ← 5 个 Custom Tool 定义
    ├── client.go           ← ModClient (HTTP → Bridge)
    ├── events.go           ← SSE 客户端
    ├── types.go            ← Go 类型定义
    └── prompt.go           ← System prompt

cmd/
└── spire2mind/
    └── main.go             ← 入口
```

## 1. Harness 模式

open-agent-sdk-go 内部的核心循环（Agent.runLoop）：

```
for turn < maxTurns {
    1. 调 LLM API（带工具 schema + 对话历史）
    2. 流式接收响应，收集 tool_use blocks
    3. 没有 tool_use → break（LLM 认为任务完成）
    4. 执行工具（RunTools，自动并发/串行分区）
    5. 工具结果追加到消息历史
    → 回到 1
}
```

spire2mind 不需要自己写这个循环，只需要：
- 定义 Custom Tools（游戏操作）
- 写 System Prompt（告诉 LLM 怎么玩）
- 启动 Agent

## 2. Custom Tools

5 个工具，每个实现 `types.Tool` 接口：

### get_game_state

```go
Name: "get_game_state"
Description: "读取当前游戏状态快照"
IsConcurrencySafe: true   // 只读，可并发
IsReadOnly: true

Call() → ModClient.GetState() → GET /state → 返回 JSON
```

### get_actions

```go
Name: "get_available_actions"
Description: "列出当前可执行的动作及其参数要求"
IsConcurrencySafe: true
IsReadOnly: true

Call() → ModClient.GetActions() → GET /actions/available → 返回动作列表
```

### act

```go
Name: "act"
Description: "执行一个游戏动作。action 必须在 available_actions 中"
InputSchema:
  action: string (必填)
  card_index: integer (可选)
  target_index: integer (可选)
  option_index: integer (可选)
IsConcurrencySafe: false  // 写操作，必须串行
IsReadOnly: false

Call() → ModClient.Act() → POST /action → 返回动作结果 + 新状态
```

### wait_until_actionable

```go
Name: "wait_until_actionable"
Description: "等待直到游戏进入可操作状态，返回最新状态"
InputSchema:
  timeout_seconds: number (可选，默认 20)
IsConcurrencySafe: false
IsReadOnly: true

Call() → ModClient.WaitUntilActionable() → 三级降级等待
```

### get_game_data

```go
Name: "get_game_data"
Description: "查询游戏元数据（卡牌/怪物/遗物/药水等）"
InputSchema:
  collection: string (cards/monsters/relics/potions/events)
  item_id: string
IsConcurrencySafe: true
IsReadOnly: true

Call() → 从本地 data/eng/*.json 查询，不需要调 Bridge
```

### 工具并发规则

open-agent-sdk-go 的 Executor 自动处理：

- `IsConcurrencySafe=true` 的工具（get_game_state, get_actions, get_game_data）→ 并发执行
- `IsConcurrencySafe=false` 的工具（act, wait_until_actionable）→ 串行执行

## 3. ModClient

### HTTP 客户端

```go
type ModClient struct {
    baseURL    string           // "http://127.0.0.1:8080"
    httpClient *http.Client
}

func (c *ModClient) GetState(ctx context.Context) (*GameState, error)
func (c *ModClient) GetActions(ctx context.Context) ([]ActionDescriptor, error)
func (c *ModClient) Act(ctx context.Context, action string, params map[string]any) (*ActionResult, error)
func (c *ModClient) WaitUntilActionable(ctx context.Context, timeout float64) (*WaitResult, error)
```

### WaitUntilActionable：三级降级

来自 STS2-Agent 的 Python 实现，用 Go 重写：

```
第一级：直接检查
  GET /state → available_actions 不为空？→ 直接返回
  （0ms 延迟）

第二级：SSE 等待
  GET /events/stream → 监听 player_action_window_opened 等事件
  收到匹配事件 → GET /state → 返回最新状态
  （通常 1-5 秒）

第三级：轮询降级
  SSE 连接失败时 → 每 250ms 轮询 GET /state
  （兜底机制）
```

设计原则：SSE 是优化手段而非必要条件，任何一环出问题都有降级路径。

### SSE 客户端

```go
func (c *ModClient) Subscribe(ctx context.Context) <-chan Event {
    ch := make(chan Event, 64)
    go func() {
        defer close(ch)
        for {
            err := c.streamEvents(ctx, ch)
            if ctx.Err() != nil { return }
            time.Sleep(time.Second) // 断线重连
        }
    }()
    return ch
}
```

SSE 事件通过 Go channel 送出，自动断线重连。

## 4. bubbletea TUI 对接

### 核心问题

bubbletea 是同步的 Elm Architecture（Update → View 循环），Agent SDK 是异步的后台 goroutine。通过 `tea.Cmd` 桥接。

### 桥接方式

```go
// 启动 Agent → 拿到 eventCh
func (m model) startAgent() tea.Cmd {
    events, errs := m.agent.Query(ctx, prompt)
    return listenAgent(events, errs)
}

// 监听 Agent 的下一个事件 → 转换为 tea.Msg
func listenAgent(events <-chan types.SDKMessage, errs <-chan error) tea.Cmd {
    return func() tea.Msg {
        select {
        case ev, ok := <-events:
            if !ok { return agentDoneMsg{} }
            return agentEventMsg{event: ev}
        case err := <-errs:
            return agentErrorMsg{err: err}
        }
    }
}

// Update 中处理事件
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case agentEventMsg:
        m.updateFromEvent(msg.event)
        return m, listenAgent(events, errs) // 继续监听下一个
}
```

`tea.Cmd` 是一个返回 `tea.Msg` 的函数。`listenAgent` 返回的 Cmd 阻塞在 channel 读取上，直到 Agent 发出下一个事件。bubbletea 在后台执行这个 Cmd，收到 Msg 后触发 Update 更新界面。

### TUI 展示内容

- 游戏状态：画面、HP、能量、金币、手牌（来自 get_game_state 的 tool_result）
- Agent 日志：LLM 的思考过程 + 工具调用记录（来自 assistant 消息）
- 统计信息：Token 消耗、费用、轮次（来自 result 消息）

## 5. System Prompt

```go
const systemPrompt = `你是一个杀戮尖塔 2 的 AI 玩家。通过工具与游戏交互。

核心循环：
1. 调 get_game_state 看当前状态
2. 根据 available_actions 决策
3. 调 act 执行动作
4. 动作后检查返回的状态
5. 如果需要等待（敌人回合等），调 wait_until_actionable

规则：
- 只执行 available_actions 中存在的动作
- 每次动作后重新读状态，不复用旧索引
- Modal 弹窗优先处理（confirm_modal / dismiss_modal）
- 查不确定的卡牌/怪物/遗物效果时用 get_game_data`
```

## 6. 入口

```go
// cmd/spire2mind/main.go

func main() {
    mod := game.NewModClient("http://127.0.0.1:8080")
    tools := game.NewGameTools(mod)

    a := agent.New(agent.Options{
        Model:          "sonnet-4-6",
        MaxTurns:       500,
        SystemPrompt:   game.SystemPrompt,
        CustomTools:    tools,
        PermissionMode: types.PermissionModeBypassPermissions,
    })
    defer a.Close()

    m := tui.NewModel(a)
    p := tea.NewProgram(m)
    if _, err := p.Run(); err != nil {
        log.Fatal(err)
    }
}
```

## 7. 未来扩展

### 自写 Harness 模式（省 Token）

当前用 SDK 的 harness（每步都经过 LLM），未来可以加一个自写的 Agent Loop：

- 规则引擎处理简单决策（确认弹窗、开箱、血厚走精英）
- 只在复杂战斗时调 LLM
- SSE 事件直接驱动循环，不需要 MCP 协议

两种模式通过命令行切换：

```bash
spire2mind agent   # 自写 harness，省 token
spire2mind sdk     # SDK harness，简单但费 token
```
