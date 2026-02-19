# Volume Control Capability

## ADDED Requirements

### Requirement: 系统 MUST 支持设置播放音量

用户 SHALL 能够通过语音指令设置或调节播放音量。

#### Scenario: 设置绝对音量

- Given 用户唤醒 PiBuddy
- When 用户说"把音量设为50"
- Then 系统将音量设置为50%
- And 系统回复"音量已设为50"

#### Scenario: 相对调大音量

- Given 当前音量为50%
- When 用户说"音量调大"
- Then 系统将音量增加到60%
- And 系统回复"音量已调到60"

#### Scenario: 相对调小音量

- Given 当前音量为50%
- When 用户说"音量调小一点"
- Then 系统将音量降低到40%
- And 系统回复"音量已调到40"

### Requirement: 系统 MUST 支持静音和取消静音

用户 SHALL 能够一键静音或取消静音。

#### Scenario: 静音

- Given 当前未静音
- When 用户说"静音"
- Then 系统设置静音
- And 系统回复"已静音"

#### Scenario: 取消静音

- Given 当前已静音
- When 用户说"取消静音"
- Then 系统取消静音
- And 系统回复"已取消静音"

### Requirement: 系统 MUST 支持查询当前音量

用户 SHALL 能够询问当前音量设置。

#### Scenario: 查询音量

- Given 当前音量为70%
- When 用户说"现在音量是多少"
- Then 系统回复"当前音量是70"

#### Scenario: 静音状态查询

- Given 当前已静音
- When 用户说"现在音量是多少"
- Then 系统回复"当前已静音"
