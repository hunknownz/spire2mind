# AIRI TTS / Prompt Research

日期：2026-04-09

## 结论

对于当前 `Spire2Mind` 这条链，TTS 最合理的分工不是“纯 Go 内部做神经语音合成”，而是：

1. Go 负责：
   - 生成主播解说文本
   - 管理 TTS 队列、优先级、打断、缓存
   - 调外部 TTS 引擎
   - 播放音频或把音频交给播放器
2. TTS 合成由外部引擎负责：
   - 本地模型
   - 局域网模型
   - OpenAI-compatible `/audio/speech`
   - CLI / HTTP 服务

这和 `AIRI` 的设计一致。`AIRI` 也不是把 TTS 深度耦合在“角色提示词”里，而是把：

- 人格表达
- 文本切分
- 情绪标签
- 语音参数
- 播放队列

拆成多层。

## AIRI 在“表达优化”上的做法

### 1. 人格提示词和语音不是一回事

`AIRI` 的表达优化，核心不是“给 TTS 一个更花的 prompt”，而是先把角色人格和说话习惯写死，再把文本交给后续语音层。

相关文件：

- `research/airi/services/telegram-bot/src/prompts/personality-v1.velin.md`
- `research/airi/packages/stage-ui/src/constants/prompts/system-v2.ts`

能看到几个关键点：

1. 先用人格提示词定义角色：
   - 不是 helpful assistant
   - 有明确情绪强度
   - 有表达习惯和立场
2. 系统提示里显式列出情绪维度
3. 让 LLM 先产出“像角色说出来的话”，不是先考虑 TTS

对我们当前项目的启发：

- `streamer` 提示词应该先产出“主播人格下的自然解说文本”
- TTS 只负责把这段文本说出来
- 不要把“情绪表达”全部指望交给语音引擎后处理

### 2. AIRI 用“消息切分”改善说话节奏

相关文件：

- `research/airi/services/telegram-bot/src/prompts/message-split-v1.velin.md`

这里的关键思路不是多说，而是：

- 兴奋、停顿、犹豫、补一句时可以拆消息
- 正常完整想法不要人工碎片化
- 切分服务于“像真人说话的节奏”

这对我们很重要，因为当前主播场景是：

- 战况瞬间变化
- 惊险、松一口气、意外、上头

这些内容非常适合在 TTS 前做“短句分段”，而不是一整段长文直接念。

对我们当前项目的启发：

- `tts_text` 应该是 2 到 4 个自然短句
- 后续可以把 `commentary` 再切成 1 到 3 个播报段
- 这比单纯加感叹号更接近“主播感”

### 3. AIRI 把上下文压成稳定格式，而不是胡乱拼 prompt

相关文件：

- `research/airi/packages/stage-ui/src/stores/chat/context-prompt.ts`

它把上下文做成稳定 XML 结构，只保留必要文本，不带噪声元数据。

这对我们当前项目的启发：

- `streamer` 上下文不要塞：
  - 当前目标
  - 房间目标
  - 下一步操作
  - 菜单步骤
- 应该塞：
  - 当前局面
  - 这拍危险还是轻松
  - 最近局势变化
  - 当前是高压、转机、惊喜还是烂运气

### 4. AIRI 的语音层是独立 pipeline

相关文件：

- `research/airi/packages/pipelines-audio/src/speech-pipeline.ts`
- `research/airi/packages/pipelines-audio/src/processors/tts-chunker.ts`
- `research/airi/packages/stage-ui/src/stores/modules/speech.ts`
- `research/airi/packages/stage-ui/src/services/speech/pipeline-runtime.ts`
- `research/airi/packages/stage-ui/src/libs/audio/manager.ts`
- `research/airi/packages/stage-ui/src/stores/audio.ts`

它做的不是“文本来了就直接念”，而是：

- 文本分段
- 多段并发生成
- 按优先级排队
- 支持 interrupt / replace / queue
- 播放器和生成器解耦
- 支持 SSML / pitch / rate / voice

这对我们当前项目的启发：

- 现在 `scratch/tts/latest.json` 只是最小版本
- 下一步应该补成真正的 `speech pipeline`
- 尤其要支持：
  - 普通解说排队
  - 高危战况插队
  - 新局面打断旧解说

### 5. AIRI 的本地播放不是 watcher 脚本，而是内存里的音频播放链

相关文件：

- `research/airi/packages/stage-ui/src/libs/audio/manager.ts`
- `research/airi/packages/stage-ui/src/stores/audio.ts`

它的做法很简单但很对：

- 拿到语音结果后，直接在内存里播放
- 用 `AudioContext.decodeAudioData(...)` 解码音频
- 用 `createBufferSource()` 播放
- 播放时把音频接到 `AnalyserNode`
- 顺手驱动“嘴型/音量”一类可视化

这说明 `AIRI` 在“播放层”的设计核心不是复杂播放器，而是：

- 语音生成和播放解耦
- 播放是标准化能力
- 音量分析是附加能力

对我们当前项目的启发：

- 现有 `watch-tts-queue.ps1` 更像一个过渡方案
- 长期更合理的是：
  - `SpeechDirector` 生成或拿到 wav/mp3
  - 直接本地播放
  - 如果以后要做主播头像或波形，再加 analyser/可视化

## AIRI 支持的 TTS 接法

从仓库里能直接看到，`AIRI` 支持的不是单一 TTS 引擎，而是一层 provider 抽象。

相关文件：

- `research/airi/packages/stage-pages/src/pages/settings/providers/speech/kokoro-local.vue`
- `research/airi/packages/stage-pages/src/pages/settings/providers/speech/openai-compatible-audio-speech.vue`
- `research/airi/README.md`

可以确认的方向包括：

- `kokoro-local`
- `openai-compatible-audio-speech`
- `OpenAI audio speech`
- `Microsoft speech`
- `ElevenLabs`
- `Deepgram`

另外 `README` 明确提到：

- `unspeech`
  - 一个统一 `/audio/transcriptions` 和 `/audio/speech` 的代理层

这说明 `AIRI` 的真正策略不是把 TTS 固定在某个框架，而是：

- 上层业务只认识“语音能力”
- 下层可以换 provider

## Go 能不能做 TTS

能，但要分清楚“做哪一层”。

### Go 适合做的

Go 非常适合做：

- TTS 调度
- 队列
- 优先级
- 打断逻辑
- 文件缓存
- HTTP 调用外部引擎
- 音频播放封装

### Go 不适合现在直接做的

Go 当前并不适合拿来做“主流神经 TTS 模型推理框架”的主战场。

原因：

- 纯 Go 的成熟神经 TTS 生态很弱
- 真正好用的 TTS 模型实现大多在：
  - Python
  - Rust/C++
  - ONNX Runtime
  - 专门 HTTP 服务

所以当前最合理的方式是：

- Go 做 orchestration
- 模型推理由外部引擎完成

## 当前适合我们的开源方案

### 1. Kokoro

仓库：

- [hexgrad/kokoro](https://github.com/hexgrad/kokoro)

为什么适合：

- `AIRI` 已经直接支持 `kokoro-local`
- 轻量
- 集成成本低
- 做“年轻、偏二次元、偏清亮”的女声更自然

局限：

- 生态还在生长中
- 中文表现要看具体 voice 和部署方式

适合用途：

- 本地/局域网测试
- 主播女声第一候选

### 2. Piper

仓库：

- [rhasspy/piper](https://github.com/rhasspy/piper)

为什么适合：

- 部署简单
- 本地推理稳定
- 做 fallback 非常合适

局限：

- 情绪和表现力不如更新一代的模型
- 更像“可靠播报”，不太像“会演的主播”

适合用途：

- fallback TTS
- 离线、稳、低依赖

### 3. MeloTTS

仓库：

- [MyShell-AI/MeloTTS](https://github.com/myshell-ai/MeloTTS)

为什么适合：

- 多语言
- 中文相对友好
- 比传统播报更自然

局限：

- 工程接入一般还是走 Python/服务端
- 不是 Go 原生

适合用途：

- 中文女声主线候选
- 如果我们要主播中文表达，这是值得重点试的

### 4. VOICEVOX

仓库：

- [VOICEVOX/voicevox_engine](https://github.com/VOICEVOX/voicevox_engine)

为什么适合：

- “动漫感女声”很强
- HTTP 引擎模式适合被 Go 调

局限：

- 日语最强
- 中文主播不一定合适

适合用途：

- 如果以后要做日语/二次元角色化更强的路线

### 5. Coqui TTS

仓库：

- [coqui-ai/TTS](https://github.com/coqui-ai/TTS)

为什么值得看：

- 历史上生态大、模型多
- 研究价值高

局限：

- 更像研究/实验平台
- 当前项目不建议它当第一接入目标

适合用途：

- research
- 不适合作为我们当前第一条主播语音主线

## 对当前 Spire2Mind 的建议

### 结论

当前最适合我们的不是“找一个纯 Go TTS 框架”，而是做一层 Go 的 `SpeechDirector`，下面挂可替换 provider。

推荐顺序：

1. 主线：
   - Go -> OpenAI-compatible `/audio/speech`
   - 背后接 `Kokoro` 或 `MeloTTS`
2. fallback：
   - Go -> `Piper`
3. 如果后面要做统一代理：
   - Go -> `unspeech`

### 为什么这样最适合当前 harness

因为我们现在已经有：

- `streamer` 负责生成情绪文本
- `scratch/tts/` 队列
- 单实例 live run

缺的不是“大模型再说一遍”，缺的是：

- 一个真正可打断、可排队、可替换 provider 的语音层

所以当前合理的系统分工应该是：

1. `streamer` 只产出：
   - mood
   - commentary
   - game_insight
   - life_reflection
   - tts_text
2. `SpeechDirector` 负责：
   - 按优先级入队
   - 分句
   - 打断/替换
   - 调 provider
   - 播放
3. provider 只负责：
   - 把文本转成音频

## 直接可执行的下一步

### Phase 1

不动主业务，只补 Go 侧语音抽象：

- `SpeechProvider` interface
- `SpeechDirector`
- `Queue / interrupt / replace`
- `Audio playback`

### Phase 2

先接一个最容易试的：

- `OpenAI-compatible /audio/speech`

这样以后：

- Kokoro
- MeloTTS
- unspeech

都能挂在同一个接口后面。

### Phase 3

再做主播表达增强：

- 分句
- 情绪优先级
- 危险局面插播
- 低优先级旧解说自动取消

## 和 streamer prompt 的关系

`AIRI` 的经验很明确：

- 表达质量先靠人格提示词和文本组织
- 说话节奏靠切分
- 语音风格靠 voice / SSML / pitch / rate
- 不要把所有“情绪价值”都压在 TTS 引擎身上

所以对 `Spire2Mind` 来说：

- `streamer prompt` 要先改对
- TTS provider 是第二层
- `SpeechDirector` 是第三层

当前正确顺序是：

1. 把 `streamer` 改成“情绪化战况解说”
2. 把 Go 的语音 pipeline 搭起来
3. 再接一个合适的女声 provider

## 本次调研用到的主要来源

- `research/airi/services/telegram-bot/src/prompts/personality-v1.velin.md`
- `research/airi/services/telegram-bot/src/prompts/message-split-v1.velin.md`
- `research/airi/packages/stage-ui/src/constants/prompts/system-v2.ts`
- `research/airi/packages/stage-ui/src/stores/chat/context-prompt.ts`
- `research/airi/packages/pipelines-audio/src/speech-pipeline.ts`
- `research/airi/packages/pipelines-audio/src/processors/tts-chunker.ts`
- `research/airi/packages/stage-ui/src/stores/modules/speech.ts`
- `research/airi/packages/stage-ui/src/services/speech/pipeline-runtime.ts`
- `research/airi/packages/stage-pages/src/pages/settings/providers/speech/kokoro-local.vue`
- `research/airi/packages/stage-pages/src/pages/settings/providers/speech/openai-compatible-audio-speech.vue`
- `research/airi/README.md`
