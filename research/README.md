# Research 调研项目

以下项目通过 symlink 引用，用于持续调研和参考。

## 项目列表

| 项目 | 说明 | 借鉴内容 |
|------|------|---------|
| [STS2-Agent](STS2-Agent) | 杀戮尖塔 2 AI Mod + MCP Server (C# + Python) | Mod 层主要参考：线程模型、HTTP API、SSE 事件流、状态构建、动作执行 |
| [STS2MCP](STS2MCP) | 杀戮尖塔 2 AI Mod + MCP Server (C# + Python) | Markdown 双格式输出、Harmony InstantMode、抽牌堆打乱、ConcurrentQueue 线程模型 |
| [open-agent-sdk-go](open-agent-sdk-go) | Go Agent SDK (harness 模式) | Agent Loop、工具执行器、流式输出、成本追踪 |
| [bubbletea](bubbletea) | Go TUI 框架 (Elm Architecture) | 终端界面 |
| [claude-code](claude-code) | Claude Code 源码 | harness 架构模式参考（queryLoop、toolOrchestration） |
