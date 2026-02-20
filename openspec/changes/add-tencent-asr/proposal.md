# Change: 添加腾讯云 ASR 支持（多层兜底）

## Why

当前 sherpa-onnx 离线 ASR 识别率一般，存在同音字错误问题（如"断桥残学"应为"断桥残雪"）。腾讯云 ASR 识别率更高，且每月有独立免费额度（一句话识别 5000 次 + 实时语音识别 5 小时），可作为主引擎，离线引擎作为兜底。

## What Changes

- **ADDED** 腾讯云一句话识别引擎（HTTP POST，≤60s 音频）
- **ADDED** 腾讯云实时语音识别引擎（WebSocket 流式，可选）
- **ADDED** 多层兜底机制：一句话识别 → 实时语音识别 → sherpa-onnx
- **ADDED** 自动切换逻辑：额度耗尽或网络错误时自动降级
- **MODIFIED** ASR 模块架构：抽象为引擎接口，支持多种实现

## Impact

- Affected specs: `asr`
- Affected code:
  - `internal/asr/` - 重构为引擎接口 + 多实现
  - `internal/config/config.go` - 新增 ASR 配置
  - `configs/pibuddy.yaml` - 新增腾讯云 ASR 配置
  - `internal/pipeline/pipeline.go` - 使用新 ASR 接口
