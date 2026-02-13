## ADDED Requirements

### Requirement: 关键词文件配置

系统 SHALL 支持通过 `keywords_file` 配置项指定关键词文件路径，关键词文件需为 sherpa-onnx 要求的拼音格式。

#### Scenario: 使用关键词文件
- **WHEN** 用户配置 `wake.keywords_file` 指向有效的关键词文件
- **THEN** 系统 SHALL 加载该文件用于唤醒词检测

#### Scenario: 关键词文件不存在
- **WHEN** 用户配置的 `keywords_file` 路径不存在
- **THEN** 系统 SHALL 在启动时返回明确的文件不存在错误

### Requirement: 关键词文件生成

系统 SHALL 提供关键词文件生成脚本，将原始关键词转换为 sherpa-onnx 要求的拼音格式。

#### Scenario: 生成关键词文件
- **WHEN** 用户运行关键词生成脚本并提供原始关键词
- **THEN** 脚本 SHALL 输出符合 sherpa-onnx 格式的关键词文件

## REMOVED Requirements

### Requirement: 直接关键词字符串配置

**Reason**: sherpa-onnx 关键词检测不接受原始文本字符串，需要经过 `text2token` 工具处理为拼音格式。

**Migration**: 用户应使用 `keywords_file` 配置项指定预先生成的关键词文件。
