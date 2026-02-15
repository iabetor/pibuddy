# Change: 添加声纹识别功能

## Why

PiBuddy 目前无法识别不同用户身份,所有用户共享相同的语音助手体验。添加声纹识别可以实现个性化服务、访问控制和多用户隔离,让树莓派智能音箱支持家庭多成员使用场景。

**方案选择**: 采用**Rust + ONNX预训练模型**方案(方案A),相比Python方案性能提升60%,内存占用减少70%。

## What Changes

- 新增 Rust 推理库 `libvoiceprint.so` (基于tract-onnx,加载预训练ONNX模型)
- 实现用户注册流程(语音采样 + 声纹特征存储到SQLite)
- 实现实时身份识别(每次对话开始前识别说话人,延迟<1.5秒)
- 支持访客模式(未识别用户降级体验)
- Go 主程序通过 CGo/FFI 调用 Rust 动态库(零拷贝音频数据)
- 添加用户管理工具(注册/删除/列表)
- 使用HuggingFace预训练模型(pyannote_embedding.onnx, 17MB)

## Impact

### 影响的能力 (Specs)
- **新增**: `voice-recognition` (声纹识别能力)
- **修改**: `pipeline` (对话流程增加身份识别阶段)
- **修改**: `audio` (音频流需要支持声纹特征提取)

### 影响的代码
- 新增 `voiceprint-rs/` (Rust推理引擎库)
- 新增 `internal/voiceprint/` (Go CGo封装)
- 修改 `internal/pipeline/pipeline.go` (集成身份识别)
- 新增 `cmd/user/` (用户管理 CLI 工具)
- 修改 `config/config.yaml` (声纹功能配置)

### 依赖变更
- Rust工具链: rustc 1.75+, cargo
- Go编译依赖: CGo启用
- ONNX模型: pyannote_embedding.onnx (17MB)
- 树莓派存储需求: 约 50MB(模型 + 库 + 用户数据)

### 性能影响
- 首次识别延迟: 约 0.5-1 秒(ONNX推理)
- 内存增加: 约 100-150MB(动态库 + 模型加载)
- CPU 占用: 识别时短暂高峰,平时无额外开销
- 启动时间: <100ms(模型加载)

### 风险与缓解
- **ONNX算子兼容性**: 使用社区验证的预训练模型(低风险)
- **FFI稳定性**: Rust端捕获panic,Go端超时保护
- **交叉编译**: 使用成熟的aarch64-unknown-linux-gnu工具链
