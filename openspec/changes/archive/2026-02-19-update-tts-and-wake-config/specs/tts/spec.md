## ADDED Requirements

### Requirement: 腾讯云 TTS 引擎

系统 SHALL 支持腾讯云 TTS 作为语音合成引擎，作为中国大陆用户的默认选项。

#### Scenario: 腾讯云 TTS 合成成功
- **WHEN** 用户配置 `tts.engine: tencent` 并提供有效的腾讯云 API 密钥
- **THEN** 系统 SHALL 调用腾讯云 TTS API 合成语音并播放

#### Scenario: 腾讯云 TTS 配置缺失
- **WHEN** 用户配置 `tts.engine: tencent` 但未提供 API 密钥
- **THEN** 系统 SHALL 在启动时返回明确的配置错误提示

### Requirement: 腾讯云 TTS 配置项

系统 SHALL 支持以下腾讯云 TTS 配置项：
- `secret_id`: 腾讯云 API SecretId
- `secret_key`: 腾讯云 API SecretKey
- `voice_type`: 音色 ID（默认 1001）
- `region`: 服务区域（默认 ap-guangzhou）

#### Scenario: 使用默认音色
- **WHEN** 用户未指定 `voice_type`
- **THEN** 系统 SHALL 使用默认音色 1001（智瑜女声）

#### Scenario: 自定义音色
- **WHEN** 用户指定有效的 `voice_type`
- **THEN** 系统 SHALL 使用指定的音色进行合成

## MODIFIED Requirements

### Requirement: TTS 引擎选择

系统 SHALL 支持多种 TTS 引擎，通过配置项 `tts.engine` 选择：
- `tencent`: 腾讯云 TTS（推荐国内用户）
- `edge`: Edge TTS（需要国际网络）
- `piper`: Piper TTS（离线）

#### Scenario: 选择腾讯云引擎
- **WHEN** 用户配置 `tts.engine: tencent`
- **THEN** 系统 SHALL 使用腾讯云 TTS 进行语音合成

#### Scenario: 选择 Edge 引擎
- **WHEN** 用户配置 `tts.engine: edge`
- **THEN** 系统 SHALL 使用 Edge TTS 进行语音合成

#### Scenario: 选择 Piper 引擎
- **WHEN** 用户配置 `tts.engine: piper`
- **THEN** 系统 SHALL 使用 Piper TTS 进行语音合成
