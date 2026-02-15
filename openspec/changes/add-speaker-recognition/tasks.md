# 实施任务清单（sherpa-onnx 方案）

方案变更：弃用 Python + gRPC 方案，改用项目已有的 `sherpa-onnx-go v1.12.24`
内置的 `SpeakerEmbeddingExtractor` 和 `SpeakerEmbeddingManager` API。
零新 CGo 依赖，纯 Go + SQLite 实现。

## 1. 基础设施
- [x] 1.1 添加 `modernc.org/sqlite` 纯 Go SQLite 驱动
- [x] 1.2 添加 `VoiceprintConfig` 到 `internal/config/config.go`
- [x] 1.3 更新 `configs/pibuddy.yaml` 添加 voiceprint 配置段

## 2. 声纹核心模块 (`internal/voiceprint/`)
- [x] 2.1 `extractor.go` — 封装 sherpa SpeakerEmbeddingExtractor
- [x] 2.2 `store.go` — SQLite 持久化用户和 embedding 数据
- [x] 2.3 `manager.go` — 编排层，统一 Identify/Register/Delete 入口

## 3. Pipeline 集成
- [x] 3.1 修改 `internal/llm/context.go` 添加 SetCurrentSpeaker
- [x] 3.2 修改 `internal/pipeline/pipeline.go` 集成声纹识别流程
  - 唤醒后缓冲音频 → 提取 embedding → 搜索匹配 → 注入 LLM 上下文
  - 优雅降级：声纹功能失败不影响主流程

## 4. 用户管理工具
- [x] 4.1 创建 `cmd/user/main.go`（register / list / delete 子命令）
- [x] 4.2 更新 Makefile 添加 `build-user` 目标

## 5. 模型下载
- [x] 5.1 更新 `scripts/setup.sh` 下载声纹模型
- [x] 5.2 更新 `scripts/setup-mac.sh` 下载声纹模型

## 6. 测试
- [x] 6.1 `internal/voiceprint/store_test.go` — SQLite CRUD 和序列化测试
- [x] 6.2 `internal/config/config_test.go` — VoiceprintConfig 默认值断言
- [x] 6.3 `internal/llm/context_test.go` — SetCurrentSpeaker 测试

## 模型
- 文件: `3dspeaker_speech_campplus_sv_zh-cn_16k-common.onnx`
- 来源: sherpa-onnx releases (speaker-recongition-models)
- 输入: 16kHz mono float32 音频
- 输出: embedding 向量（维度由模型决定）
