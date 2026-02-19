## 1. Pipeline struct 改造

- [x] 1.1 在 `Pipeline` 结构体中添加 `interrupted atomic.Bool` 字段
- [x] 1.2 引入 `sync/atomic` 包

## 2. processFrame 支持 Processing 状态打断

- [x] 2.1 新增 `handleProcessingInterrupt(ctx, frame)` 方法，逻辑与 `handleSpeakingInterrupt` 类似，在 `StateProcessing` 下检测唤醒词
- [x] 2.2 在 `processFrame` 的 `StateProcessing` 分支调用 `handleProcessingInterrupt`

## 3. 打断机制重构

- [x] 3.1 `handleSpeakingInterrupt` 和 `handleProcessingInterrupt` 中设置 `p.interrupted.Store(true)`
- [x] 3.2 打断后播放 `InterruptReply`，进入监听并启动连续对话计时器

## 4. processQuery 响应打断

- [x] 4.1 `processQuery` 入口处重置 `p.interrupted.Store(false)`
- [x] 4.2 在 textCh 循环、sentenceBuf 提取循环、工具调用循环等关键节点检查 `p.interrupted.Load()`，为 true 时排空 channel 并 return
- [x] 4.3 替换原有的状态检查（`state == Idle || state == Listening`）为 `p.interrupted.Load()` 检查

## 5. speakText 不再自动切状态

- [x] 5.1 移除 `speakText` 末尾的 `if state == Speaking { SetState(Processing) }` 逻辑
- [x] 5.2 在 `processQuery` 中需要切 Processing 的地方显式调用（进入工具调用轮次前）

## 6. 回复语后延迟监听

- [x] 6.1 `DialogConfig` 添加 `ListenDelay int` 字段（毫秒），默认 500ms
- [x] 6.2 `pibuddy.yaml` 添加 `listen_delay: 500` 配置项
- [x] 6.3 `playWakeReply` 播放完后、进入 Listening 前 sleep `ListenDelay` 毫秒
- [x] 6.4 打断回复播放完后、进入 Listening 前同样 sleep `ListenDelay` 毫秒

## 7. 测试验证

- [x] 7.1 编译通过 `go build ./...`
- [x] 7.2 确认现有测试通过 `go test ./...`
