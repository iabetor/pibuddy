## 1. TTS 腾讯云引擎

- [x] 1.1 添加腾讯云 SDK 依赖 (`github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tts`)
- [x] 1.2 实现 `TencentTTSEngine` 结构体，实现 `Engine` 接口
- [x] 1.3 更新 `TTSConfig` 配置结构体，添加腾讯云配置项
- [x] 1.4 更新 `configs/pibuddy.yaml` 默认配置
- [x] 1.5 更新 pipeline.go 支持 `tencent` 引擎

## 2. 唤醒词配置修复

- [x] 2.1 更新 `WakeConfig` 添加 `KeywordsFile` 字段
- [x] 2.2 修改 `NewDetector` 使用 `config.KeywordsFile` 替代 `config.KeywordsBuf`
- [x] 2.3 更新 `configs/pibuddy.yaml` 使用 `keywords_file` 配置项
- [x] 2.4 关键词文件已包含 "你好小派"（setup-mac.sh 已支持）

## 3. VAD 参数优化

- [x] 3.1 更新 `setDefaults` 中 `MinSilenceMs` 默认值为 1200
- [x] 3.2 更新 `configs/pibuddy.yaml` 默认配置

## 4. 文档更新

- [x] 4.1 更新 `openspec/project.md` TTS 依赖说明
- [x] 4.2 添加腾讯云 TTS 配置说明

## 5. 测试验证

- [x] 5.1 编译通过
- [x] 5.2 唤醒词检测器初始化成功
- [x] 5.3 腾讯云 TTS 引擎初始化成功
- [x] 5.4 VAD 静音检测参数已更新为 1200ms
