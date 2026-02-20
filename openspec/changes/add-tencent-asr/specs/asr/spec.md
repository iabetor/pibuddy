## ADDED Requirements

### Requirement: 多引擎 ASR 架构
系统 SHALL 支持多种 ASR 引擎，并通过统一接口进行调用。

#### Scenario: 引擎接口统一
- **WHEN** Pipeline 调用 ASR 功能
- **THEN** 系统通过统一的 ASREngine 接口调用底层引擎
- **AND** 不需要关心具体引擎实现

#### Scenario: 引擎可用性检测
- **WHEN** 检查引擎是否可用
- **THEN** 系统返回该引擎当前的可用状态（额度/网络）

---

### Requirement: 腾讯云一句话识别引擎
系统 SHALL 支持腾讯云一句话识别作为 ASR 引擎之一。

#### Scenario: 正常识别
- **WHEN** 用户说话结束（VAD 端点检测）
- **AND** 腾讯云一句话识别引擎可用
- **THEN** 系统将缓存的音频发送到腾讯云 API
- **AND** 返回识别结果文本

#### Scenario: 额度耗尽
- **WHEN** 腾讯云返回额度耗尽错误
- **THEN** 系统自动切换到下一个可用引擎
- **AND** 记录日志说明切换原因

#### Scenario: 网络错误
- **WHEN** 调用腾讯云 API 发生网络错误
- **THEN** 系统自动切换到下一个可用引擎
- **AND** 记录日志说明切换原因

---

### Requirement: 多层兜底机制
系统 SHALL 支持多层 ASR 引擎兜底，确保至少有一个引擎可用。

#### Scenario: 引擎降级
- **WHEN** 当前引擎不可用（额度耗尽或网络错误）
- **THEN** 系统自动切换到优先级更低的引擎
- **AND** 继续提供语音识别功能

#### Scenario: 所有引擎不可用
- **WHEN** 所有引擎都不可用
- **THEN** 系统返回错误，提示用户稍后重试

#### Scenario: 引擎恢复
- **WHEN** 更高优先级的引擎恢复可用
- **THEN** 系统自动切换回更高优先级的引擎
- **AND** 记录日志说明恢复情况

---

### Requirement: 离线 ASR 兜底
系统 SHALL 始终保留 sherpa-onnx 离线引擎作为最终兜底。

#### Scenario: 离线兜底激活
- **WHEN** 所有云端引擎都不可用
- **THEN** 系统使用 sherpa-onnx 离线引擎进行识别
- **AND** 无需网络连接即可工作

#### Scenario: 离线引擎始终可用
- **WHEN** 检查 sherpa-onnx 引擎可用性
- **THEN** 系统始终返回可用状态

---

## MODIFIED Requirements

### Requirement: ASR 配置
系统 SHALL 支持配置多个 ASR 引擎及其优先级。

#### Scenario: 配置引擎优先级
- **WHEN** 用户在配置文件中设置引擎列表
- **THEN** 系统按照列表顺序确定引擎优先级

#### Scenario: 配置腾讯云凭证
- **WHEN** 用户配置腾讯云 SecretId 和 SecretKey
- **THEN** 系统使用该凭证调用腾讯云 ASR API

#### Scenario: 复用现有凭证
- **WHEN** 用户已配置腾讯云 TTS 凭证
- **THEN** 系统可复用该凭证调用腾讯云 ASR API
