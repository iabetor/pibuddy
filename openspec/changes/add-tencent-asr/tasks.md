# Implementation Tasks

## 1. 基础架构重构
- [x] 1.1 创建 `internal/asr/engine.go`，定义 `Engine` 接口
- [x] 1.2 重构 `internal/asr/recognizer.go`，`SherpaEngine` 实现接口
- [x] 1.3 创建 `internal/asr/fallback.go`，实现多层兜底逻辑
- [x] 1.4 错误类型定义在 `fallback.go` 中

## 2. 配置更新
- [x] 2.1 更新 `internal/config/config.go`，新增 ASR 多引擎配置
- [x] 2.2 更新 `configs/pibuddy.yaml`，添加腾讯云 ASR 配置

## 3. 腾讯云一句话识别
- [x] 3.1 创建 `internal/asr/tencent_flash.go`，实现腾讯云一句话识别
- [x] 3.2 使用官方 SDK 调用
- [x] 3.3 实现错误码解析（额度耗尽、网络错误）
- [x] 3.4 集成到 `FallbackEngine` 作为第一层引擎

## 4. 腾讯云实时语音识别
- [x] 4.1 创建 `internal/asr/tencent_rt.go`，实现 WebSocket 实时识别
- [x] 4.2 实现 WebSocket 连接管理
- [ ] 4.3 测试验证实时语音识别功能

## 5. Pipeline 集成
- [x] 5.1 更新 `internal/pipeline/pipeline.go`，使用新 ASR 接口
- [x] 5.2 创建 `initASREngine` 函数，支持三层降级

## 6. 统一数据库
- [x] 6.1 创建 `internal/database/database.go`，统一数据库管理
- [x] 6.2 重构音乐缓存 `internal/audio/cache.go`，使用 SQLite
- [ ] 6.3 迁移声纹存储到统一数据库（可选）

## 7. 测试与验证
- [ ] 7.1 单元测试：各引擎接口实现
- [ ] 7.2 集成测试：引擎切换逻辑
- [ ] 7.3 端到端测试：实际语音识别效果

## 8. 优化（可选 - Phase 3）
- [ ] 8.1 定期恢复到更优引擎（每 5 分钟检查）
- [ ] 8.2 使用统计：记录各引擎调用次数到 SQLite
- [ ] 8.3 热词支持：利用腾讯云 HotwordList 提升歌曲名/歌手名识别率
