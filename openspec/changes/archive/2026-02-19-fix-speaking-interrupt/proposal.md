# Change: 修复长回复播放时无法唤醒词打断

## Why

当 LLM 生成长回复（如讲故事）时，用户说唤醒词无法打断播放。根因是 `processQuery` 按句逐句合成播放，每句播放完 `speakText` 会将状态从 `Speaking` 切回 `Processing`，而 `processFrame` 在 `Processing` 状态下完全不做任何处理（不检测唤醒词）。在两句之间存在一段"盲区"（包含下一句的 TTS 网络合成耗时），期间唤醒词检测完全失效。加上 `cancelSpeak` 在句间也为 nil，即使检测到了也无法打断。

## What Changes

1. **在 `StateProcessing` 也启用唤醒词检测** — `processFrame` 的 `StateProcessing` 分支加入唤醒词打断处理
2. **引入"会话级"打断 flag** — 在 `Pipeline` 上增加 `interrupted` 标志位，`processQuery` 的所有循环节点检查该标志以快速退出
3. **`speakText` 不再切回 Processing** — 由 `processQuery` 自行管理状态转换，`speakText` 只负责播放
4. **打断后播放 `InterruptReply`** — 打断成功后播放"我在"并进入监听
5. **打断/唤醒后延迟进入监听** — 播放回复语后短暂等待（如 500ms），给用户反应时间再开始监听，避免用户还没准备好说话就已经在识别了

## Impact

- Affected code:
  - `internal/pipeline/pipeline.go` — processFrame, speakText, processQuery, handleSpeakingInterrupt, playWakeReply, Pipeline struct
  - `internal/config/config.go` — DialogConfig 添加 ListenDelay 字段
  - `configs/pibuddy.yaml` — 添加 listen_delay 配置
- Affected specs: pipeline
- 无 breaking change，只改善现有行为
