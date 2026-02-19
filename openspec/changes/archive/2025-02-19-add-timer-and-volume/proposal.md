# 提案：添加倒计时器和音量控制功能

## Why

PiBuddy 作为一个智能语音助手，目前缺少两个核心的日常生活功能：

1. **倒计时器** - 用户在厨房烹饪、工作学习等场景下需要计时提醒，这是智能音箱的高频使用场景
2. **音量控制** - 用户在不同环境（白天/夜晚、安静/嘈杂）下需要调节音量，语音控制比手动操作更便捷

这两个功能实现成本低、用户价值高，能够显著提升 PiBuddy 的实用性。

## 概述

为 PiBuddy 添加两个实用功能：
1. **倒计时器** - 语音设置倒计时，到时语音提醒
2. **音量控制** - 语音调节播放音量

## 功能描述

### 1. 倒计时器 (Timer)

**用户场景**：
- "设个5分钟倒计时"
- "定个10分钟的计时器"
- "帮我倒计时3分钟，提醒我关火"
- "还有多久" - 查询当前倒计时剩余时间
- "取消倒计时"

**功能设计**：
- 支持分钟和秒两种单位（"5分钟"、"30秒"、"1分30秒"）
- 倒计时结束时语音播报提醒
- 支持多个同时运行的倒计时器（最多5个）
- 支持自定义提醒内容
- 倒计时期间可查询剩余时间
- 持久化存储，重启后恢复

### 2. 音量控制 (Volume Control)

**用户场景**：
- "音量调大"
- "音量调小一点"
- "把音量设为50"
- "静音"
- "取消静音"
- "现在音量是多少"

**功能设计**：
- 音量范围：0-100
- 支持"调大/调小"相对调节（每次±10）
- 支持"设为X"绝对设置
- 支持静音/取消静音
- 支持查询当前音量
- 音量状态持久化

## 技术方案

### 倒计时器实现

复用现有 `AlarmStore` 的存储模式，创建独立的 `TimerStore`：

```
internal/tools/timer.go
├── TimerStore      - 倒计时器存储（内存 + 持久化）
├── SetTimerTool    - 设置倒计时
├── ListTimersTool  - 查看倒计时列表
├── CancelTimerTool - 取消倒计时
└── TimerManager    - 后台 goroutine 检查到期倒计时
```

**关键设计**：
- 使用 `time.AfterFunc` 实现倒计时触发
- 到期后通过 channel 通知 pipeline 播报
- 支持带标签的倒计时器（如"关火提醒"）

### 音量控制实现

通过系统调用控制音量：

```
internal/tools/volume.go
├── VolumeController - 音量控制器（跨平台抽象）
├── SetVolumeTool    - 设置音量
├── AdjustVolumeTool - 相对调节音量
├── MuteTool         - 静音/取消静音
└── GetVolumeTool    - 查询音量
```

**跨平台支持**：
- Linux (ALSA): `amixer set Master X%`
- Linux (PulseAudio): `pactl set-sink-volume @DEFAULT_SINK@ X%`
- macOS (开发测试): `osascript -e 'set volume output volume X'`

## 影响范围

- 新增文件：`internal/tools/timer.go`, `internal/tools/volume.go`
- 修改文件：`internal/pipeline/pipeline.go`（注册新工具、处理倒计时到期通知）
- 配置文件：可选添加默认音量配置

## 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 倒计时精度 | 低 | 使用 Go 标准库 timer，精度足够 |
| 音量命令兼容性 | 中 | 优先 PulseAudio，回退 ALSA |
| 多倒计时管理复杂度 | 低 | 限制最多5个同时运行 |

## 预期收益

- 提升日常实用性（厨房计时、工作提醒）
- 完善语音控制能力
- 为后续智能家居控制打基础
