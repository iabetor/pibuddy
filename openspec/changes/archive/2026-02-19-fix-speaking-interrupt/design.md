## Context

PiBuddy 的主循环在单个 goroutine 中连续读取音频帧并调用 `processFrame`。`processQuery` 在另一个 goroutine 中运行，按句合成播放。两者通过 `state` 和 `cancelSpeak` 交互。

当前问题是状态在句间切到 `Processing`，`processFrame` 完全忽略该状态的音频帧，导致唤醒词检测出现盲区。

### 根因分析

```
时间线（两句之间）：

processQuery goroutine              |  主循环 goroutine
──────────────────────────────────  |  ─────────────────────────
sentence1 播放完毕                    |
speakText 设 state=Processing        |  processFrame: Processing → 什么都不做 ❌
TTS 合成 sentence2（网络请求 0.3~2s）  |  processFrame: Processing → 什么都不做 ❌
设 state=Speaking                     |
player.Play(sentence2)               |  processFrame: Speaking → 检测唤醒词 ✅
```

盲区 = TTS 网络合成延迟 + LLM 流式等待，可达数秒。

### 补充问题

`cancelSpeak` 在 `speakText` 退出的 defer 里被置 nil。句间调用 `interruptSpeak()` 时无 cancel 可执行，也无法阻止下一次 `speakText` 启动。

## Goals / Non-Goals

- **Goal**: 在整个 `processQuery` 执行期间（包括 TTS 合成、LLM 等待、句间间隙）都能检测唤醒词并打断
- **Goal**: 打断后快速响应，播放"我在"进入监听
- **Goal**: 打断后 processQuery goroutine 干净退出，不遗留资源
- **Non-Goal**: 不改变音频采集或 Player 实现
- **Non-Goal**: 不做 TTS 预取优化（可后续独立做）

## Decisions

### Decision 1: 在 `ProcessFrame` 的 `StateProcessing` 分支也检测唤醒词

**Why**: 最直接地消除盲区。Processing 状态下音频帧本来就在被采集，只是被丢弃了。

**Alternative**: 让 speakText 不切回 Processing → 但 processQuery 需要在 Processing 和 Speaking 之间切换来支持工具调用等场景，移除这个切换会破坏其他逻辑。

### Decision 2: 用 `interrupted` atomic flag 替代纯状态检查

**Why**: `cancelSpeak` 句间为 nil，仅靠状态检查不够。`interrupted` flag 可以在任何时刻被设置，`processQuery` 的循环每次迭代都检查它，保证快速退出。

**Implementation**:
```go
// Pipeline 新增字段
interrupted atomic.Bool

// handleProcessingInterrupt / handleSpeakingInterrupt 中:
p.interrupted.Store(true)
p.interruptSpeak()

// processQuery 中所有关键节点:
if p.interrupted.Load() {
    return
}

// processQuery 入口处重置:
p.interrupted.Store(false)
```

### Decision 3: speakText 不再自动切状态

**Why**: 状态管理应该归调用者（processQuery）。speakText 末尾切 Processing 导致句间盲区。去掉后由 processQuery 在合适的时机（如进入工具调用循环前）切换。

### Decision 4: 播放回复语后延迟进入监听

**Why**: 用户打断或唤醒后，回复语播完立即进入监听，用户可能还没反应过来就已经在识别了，体验不好。在播放完回复语后 sleep 一小段时间（可配置，默认 500ms），再开始监听。

**Implementation**:
```go
// DialogConfig 新增字段
ListenDelay int `yaml:"listen_delay"` // 毫秒，默认 500

// playWakeReply / 打断回复后:
if p.cfg.Dialog.ListenDelay > 0 {
    time.Sleep(time.Duration(p.cfg.Dialog.ListenDelay) * time.Millisecond)
}
p.state.SetState(StateListening)
```

**Scope**: 适用于所有"回复语 → 监听"的转换场景，包括正常唤醒和打断。

## Risks / Trade-offs

- **Risk**: `StateProcessing` 下检测唤醒词会与正常 Processing 逻辑并行
  - **Mitigation**: Processing 期间无其他音频处理逻辑，不冲突
- **Risk**: `interrupted` flag 和状态机并存增加复杂度
  - **Mitigation**: interrupted 只用于 processQuery 内部流控，不影响外部状态机语义

## Open Questions

- 无
