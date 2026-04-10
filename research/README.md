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

## STS2 社区 Mod 生态

以下项目用于持续调研 STS2 官方 mod 机制和社区最佳实践，为 Bridge 重构提供参考。

| 项目 | 地址 | 借鉴内容 |
|------|------|---------|
| BaseLib-StS2 | [github.com/Alchyr/BaseLib-StS2](https://github.com/Alchyr/BaseLib-StS2) | CustomSingletonModel hook 机制、SpireField 状态附加、ModInterop 跨 mod 通信、ModConfig 配置系统 |
| ModTemplate-StS2 | [github.com/Alchyr/ModTemplate-StS2](https://github.com/Alchyr/ModTemplate-StS2) | 标准 mod 结构、manifest 规范、build/publish 流程、[wiki 文档](https://github.com/Alchyr/ModTemplate-StS2/wiki/Setup) |
| Minty-Spire-2 | [github.com/erasels/Minty-Spire-2](https://github.com/erasels/Minty-Spire-2) | combat hook 全量订阅（MintyHooker）、Wiz 工具类、敌人意图计算、power 查询、UI 注入、WeakNodeRegistry |
| StS2-Quick-Restart | [github.com/erasels/StS2-Quick-Restart](https://github.com/erasels/StS2-Quick-Restart) | Harmony patch 模式、AssemblyPublicizer 用法、RunManager/SaveManager 操作、游戏流程控制 |
| WatcherMod | [github.com/lamali292/WatcherMod](https://github.com/lamali292/WatcherMod) | 自定义 hook dispatch、ModelDb 注册表查询、ScriptManagerBridge 用法、完整角色 mod 架构 |
| freude916/sts2-quickRestart | [github.com/freude916/sts2-quickRestart](https://github.com/freude916/sts2-quickRestart) | 最全面的中文 modding 指南、控制台命令列表、多人架构、存档路径、启动参数 |

### 关键参考路径

**官方 mod 机制理解**: [Steam patch notes](https://steamcommunity.com/app/2868840/allnews/) → ModTemplate wiki → BaseLib README

**Bridge 重构优先参考源码**:
- `Minty-Spire-2/src/MintyHooker.cs` — CustomSingletonModel 的 12+ hook 实现
- `Minty-Spire-2/src/Wiz.cs` — 游戏状态便捷访问工具类
- `StS2-Quick-Restart/src/MainFile.cs` — RunManager/SaveManager 完整操作流程
- `StS2-Quick-Restart/StS2-Quick-Restart.csproj` — AssemblyPublicizer 配置示例
- `WatcherMod/src/WatcherHook.cs` — 自定义 hook dispatch via `IterateHookListeners()`
- `BaseLib/src/CustomSingletonModel.cs` — hook 基类实现
- `BaseLib/src/SpireField.cs` — ConditionalWeakTable 状态附加
