# Design: 腾讯云 ASR 多层兜底架构

## Context

当前 pibuddy 使用 sherpa-onnx 离线 ASR，存在以下问题：
- 识别率一般，同音字错误多
- 无法利用云端更强的模型
- 无自动纠错机制

腾讯云 ASR 提供：
- 一句话识别：5000 次/月免费，HTTP POST 调用
- 实时语音识别：5 小时/月免费，WebSocket 流式
- 更高的识别准确率

## Goals / Non-Goals

**Goals:**
- 实现多层 ASR 架构，最大化利用免费额度
- 自动切换引擎，用户无感知
- 离线兜底，保证网络问题时仍可用

**Non-Goals:**
- 不支持语音唤醒词检测的云端化（保持本地 sherpa-onnx）
- 不实现语音识别结果的持久化存储
- 不实现用户可配置的热词管理界面

## Decisions

### Decision 1: 引擎优先级

**选择**: 一句话识别 → 实时语音识别 → sherpa-onnx

**原因**:
- 一句话识别额度最多（5000 次/月），延迟可接受（200-500ms）
- 实时语音识别作为备用（5 小时/月）
- sherpa-onnx 作为最终兜底（无限制）

**备选方案**:
- 实时语音识别优先：延迟更低，但额度只有 5 小时
- 仅用一句话识别 + sherpa：简化实现，但浪费实时语音识别额度

### Decision 2: 切换时机

**选择**: 额度耗尽错误码 + 网络错误时切换

**原因**:
- 腾讯云返回 `FailedOperation.UserHasNoAmount` 表示额度耗尽
- 网络超时/连接失败表示需要离线兜底

**错误码处理**:
| 错误码 | 说明 | 处理 |
|--------|------|------|
| `FailedOperation.UserHasNoAmount` | 额度耗尽 | 切换下一引擎 |
| 网络超时 | 网络问题 | 切换下一引擎 |
| 其他错误 | 识别失败 | 返回空结果 |

### Decision 3: 恢复机制

**选择**: 每 10 分钟尝试恢复到更优引擎

**原因**:
- 额度每月 1 日重置，需要定期检查
- 网络可能临时中断，需要重试
- 用户无感知，自动优化体验

### Decision 4: 接口设计

**选择**: 定义统一的 `ASREngine` 接口

**原因**:
- 不同引擎 API 差异大（HTTP vs WebSocket vs 本地）
- 统一接口便于扩展和测试
- MultiLayerASR 作为组合模式实现

```go
type ASREngine interface {
    Name() string
    Feed(samples []float32) error
    GetResult() string
    IsEndpoint() bool
    Reset()
    Close()
    IsAvailable() bool
}
```

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|----------|
| 腾讯云 API 变更 | 封装适配层，隔离变化 |
| 一句话识别延迟 | 200-500ms 可接受，用户无感知 |
| 额度耗尽频繁切换 | 每月 5000 次足够日常使用 |
| 网络依赖 | sherpa-onnx 作为最终兜底 |

## Migration Plan

1. **Phase 1**: 重构现有 sherpa-onnx 为接口实现，不影响现有功能
2. **Phase 2**: 添加腾讯云引擎，配置中默认关闭，逐步启用
3. **Phase 3**: 验证稳定后，调整引擎优先级

**回滚**: 配置中禁用腾讯云引擎，仅保留 sherpa-onnx

## Open Questions

1. ~~是否需要实现实时语音识别？~~ → Phase 2 可选
2. 热词列表如何管理？→ 后续优化，可硬编码常用歌曲名/歌手名
