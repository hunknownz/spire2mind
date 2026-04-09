# Spire2Mind

Spire2Mind 是一套面向 Windows 版《杀戮尖塔 2》的自动游玩与研究框架。

它由三层组成：

- **Bridge（C# / Godot Mod）**：运行在游戏进程内，读取实时状态、暴露可执行动作、执行唯一合法写入。
- **Runtime（Go）**：负责主循环、稳定状态读取、动作解析、恢复、日志、TUI 和策略接线。
- **Model / Planner**：确定性规则、MCTS/planner、本地或远端大模型，用于选动作和主播解说。

当前主线目标不是“跑通接口”，而是：

- 稳定推进到更深层数
- 逐步提高通关率
- 保持可分析、可回放、可清理的运行数据
- 支持主播解说和本地 TTS 播放

## 当前能力

已实现并持续验证：

- 主菜单继续 / 新开局
- 角色选择与出发
- 地图移动
- 战斗
- 奖励界面
- 事件
- 宝箱
- 火堆
- 商店（购买 / 移除）
- 选牌类界面
- Headless smoke / soak
- Bubble Tea TUI
- 中文 streamer 解说 + 本地 TTS sidecar

## 仓库结构

- [`bridge`](C:/Users/klerc/spire2mind/bridge)
  游戏内 Bridge mod
- [`cmd/spire2mind`](C:/Users/klerc/spire2mind/cmd/spire2mind)
  主程序入口
- [`internal`](C:/Users/klerc/spire2mind/internal)
  运行时、策略、TUI、状态和 artifact 逻辑
- [`scripts`](C:/Users/klerc/spire2mind/scripts)
  启动、诊断、清理、TTS、巡检脚本
- [`docs`](C:/Users/klerc/spire2mind/docs)
  架构、运行说明、artifact 规则、streamer 方案
- [`research`](C:/Users/klerc/spire2mind/research)
  外部方案调研、参考实现、实验材料
- [`scratch`](C:/Users/klerc/spire2mind/scratch)
  运行产物、知识库、TTS 队列、临时日志

## 环境要求

本机开发环境默认是：

- Windows
- Steam 版《杀戮尖塔 2》
- .NET 9 SDK
- Go（或仓库内 `.tools` 提供的 Go）
- 已安装并信任的本地 Bridge 签名证书

如果 Windows App Control / WDAC 阻止 Bridge，先按 Bridge 脚本安装本地开发证书和补充策略。

## 快速开始

### 1. 构建并安装 Bridge

```powershell
pwsh .\bridge\scripts\build.ps1
pwsh .\bridge\scripts\install.ps1
```

### 2. 启动游戏

游戏路径默认是：

`C:\Program Files (x86)\Steam\steamapps\common\Slay the Spire 2\SlayTheSpire2.exe`

启动后等待 Bridge 健康检查通过。

### 3. 跑环境自检

```powershell
pwsh .\scripts\doctor.ps1
```

这会检查：

- Bridge 健康状态
- `/state` / `/actions/available`
- SSE
- artifacts 目录
- 模型配置与模型探针

## 运行模式

### 可见 TUI

```powershell
pwsh .\scripts\run-tui.ps1
```

常用按键：

- `p`：暂停 / 恢复
- `q`：退出
- `Ctrl+C`：停止

### Headless smoke

```powershell
pwsh .\scripts\headless-smoke.ps1 -TimeoutSeconds 300
```

### Long soak

```powershell
pwsh .\scripts\long-soak.ps1 -Attempts 3 -TimeoutSeconds 900
```

## 模型接入

Runtime 支持两类 provider：

- `claude-cli`
- `api`

当前主线更常用的是 `api`，因为可以挂：

- 局域网远端模型
- 本地 OpenAI-compatible 服务
- Ollama / 自建服务

常用脚本：

- [`start-spire2mind-api.ps1`](C:/Users/klerc/spire2mind/scripts/start-spire2mind-api.ps1)
- [`start-spire2mind-claude-cli.ps1`](C:/Users/klerc/spire2mind/scripts/start-spire2mind-claude-cli.ps1)
- [`start-spire2mind-local-llm.ps1`](C:/Users/klerc/spire2mind/scripts/start-spire2mind-local-llm.ps1)
- [`start-spire2mind-qwen35a3b-coding-nvfp4.ps1`](C:/Users/klerc/spire2mind/scripts/start-spire2mind-qwen35a3b-coding-nvfp4.ps1)

当前主线模型配置常见为：

- provider：`api`
- backend：局域网或本地 OpenAI-compatible 服务
- model：`qwen3.5:35b-a3b-coding-nvfp4`
- decision mode：`structured`

说明：

- `structured`：模型只返回结构化动作，真正执行由 harness 负责
- `tools`：模型自己调工具；保留做实验，不是当前主线

## Streamer / TTS

项目支持主播解说模式。

链路是：

1. Runtime 在关键时刻生成 `streamer beat`
2. 生成：
   - 情绪
   - 解说
   - 游戏洞察
   - 人生感慨
   - `tts_text`
   - `tts_segments`
3. sidecar 负责排队、打断、播放

当前默认本地 TTS 主线：

- provider：`melotts`
- base URL：`http://127.0.0.1:18080`
- fallback：`windows-sapi`

备选本地 provider：

- `kokoro`
- base URL：`http://127.0.0.1:18081`

相关脚本：

- [`setup-local-melotts.ps1`](C:/Users/klerc/spire2mind/scripts/setup-local-melotts.ps1)
- [`start-local-melotts.ps1`](C:/Users/klerc/spire2mind/scripts/start-local-melotts.ps1)
- [`setup-local-kokoro.ps1`](C:/Users/klerc/spire2mind/scripts/setup-local-kokoro.ps1)
- [`start-local-kokoro.ps1`](C:/Users/klerc/spire2mind/scripts/start-local-kokoro.ps1)
- [`start-tts-player.ps1`](C:/Users/klerc/spire2mind/scripts/start-tts-player.ps1)
- [`switch-tts-profile.ps1`](C:/Users/klerc/spire2mind/scripts/switch-tts-profile.ps1)

## 运行产物：哪些该保留，哪些该删

### 可长期保留、可提交的知识库

- [`scratch/guidebook/guidebook.md`](C:/Users/klerc/spire2mind/scratch/guidebook/guidebook.md)
- [`scratch/guidebook/living-codex.json`](C:/Users/klerc/spire2mind/scratch/guidebook/living-codex.json)
- [`scratch/guidebook/combat-playbook.md`](C:/Users/klerc/spire2mind/scratch/guidebook/combat-playbook.md)
- [`scratch/guidebook/event-playbook.md`](C:/Users/klerc/spire2mind/scratch/guidebook/event-playbook.md)

### 可删除重跑的日志

- `scratch/agent-runs/`
- `scratch/manual-runs/`
- `scratch/tts/queue/`
- `scratch/state-live.json`
- `scratch/state-live2.json`
- `scratch/local-llm-launch.log`

清理脚本：

```powershell
pwsh .\scripts\clean-logs.ps1
```

它会保留知识库，不会删 `scratch/guidebook/`。

## 常用脚本

- [`doctor.ps1`](C:/Users/klerc/spire2mind/scripts/doctor.ps1)
  环境 / Bridge / 模型自检
- [`run-tui.ps1`](C:/Users/klerc/spire2mind/scripts/run-tui.ps1)
  可见 TUI
- [`headless-smoke.ps1`](C:/Users/klerc/spire2mind/scripts/headless-smoke.ps1)
  短时 smoke
- [`long-soak.ps1`](C:/Users/klerc/spire2mind/scripts/long-soak.ps1)
  连续 soak
- [`watch-dashboard.ps1`](C:/Users/klerc/spire2mind/scripts/watch-dashboard.ps1)
  看最新 dashboard
- [`scan-mojibake.ps1`](C:/Users/klerc/spire2mind/scripts/scan-mojibake.ps1)
  巡检 `events.jsonl` / prompt / TTS 日志里的坏字

## 设计原则

- **单写入口**
  只有 Bridge 可以写游戏
- **状态先稳定再行动**
  Runtime 先拿稳定、可操作的 state，再解动作
- **deterministic-first / planner-first / structured decision**
  简单动作走规则，复杂战斗优先 planner，模型只做结构化决策
- **日志与知识分层**
  运行日志可删，跨局知识可保留
- **主播层旁路**
  streamer / TTS 不接管主决策回路

## 进一步阅读

- [`docs/architecture.md`](C:/Users/klerc/spire2mind/docs/architecture.md)
- [`docs/runbook.md`](C:/Users/klerc/spire2mind/docs/runbook.md)
- [`docs/streamer-mode.md`](C:/Users/klerc/spire2mind/docs/streamer-mode.md)
- [`docs/artifact-policy.md`](C:/Users/klerc/spire2mind/docs/artifact-policy.md)
- [`docs/current-baseline.md`](C:/Users/klerc/spire2mind/docs/current-baseline.md)
