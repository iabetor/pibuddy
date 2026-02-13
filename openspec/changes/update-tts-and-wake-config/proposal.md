# Change: 更新 TTS 引擎和唤醒词配置

## Why

当前存在三个问题影响用户体验：
1. **Edge TTS 在国内网络环境下不可用**，WebSocket 握手失败导致无语音回复
2. **唤醒词检测失败**，sherpa-onnx 关键词检测需要特定格式的关键词文件，而非原始文本字符串
3. **VAD 静音检测时间过短**（500ms），用户说完唤醒词后需要立即说话，否则会提前结束

## What Changes

### 1. TTS 引擎切换
- 添加腾讯云 TTS 引擎作为默认选项
- 保留 Edge TTS 和 Piper TTS 作为备选
- 新增腾讯云 TTS 配置项（SecretId、SecretKey、VoiceType）
- **BREAKING**: 需要腾讯云 API 密钥才能使用语音合成

### 2. 唤醒词配置修复
- 修改 `WakeConfig` 支持 `keywords_file` 配置项
- 使用 `KeywordsFile` 字段替代 `KeywordsBuf`
- 提供关键词文件生成脚本和文档

### 3. VAD 参数优化
- 将 `min_silence_ms` 默认值从 500ms 调整为 1200ms
- 给用户更多反应时间开始说话

## Impact

- **Affected specs**: tts, wake, vad
- **Affected code**:
  - `internal/tts/engine.go` - 添加腾讯云 TTS 引擎
  - `internal/wake/detector.go` - 使用 KeywordsFile 字段
  - `internal/config/config.go` - 更新配置结构体
  - `configs/pibuddy.yaml` - 更新默认配置
- **User action required**:
  - 配置腾讯云 API 密钥（环境变量 `PIBUDDY_TENCENT_SECRET_ID` 和 `PIBUDDY_TENCENT_SECRET_KEY`）
  - 重新生成关键词文件（已提供脚本）
