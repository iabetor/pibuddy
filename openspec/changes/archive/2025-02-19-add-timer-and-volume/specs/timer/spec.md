# Timer Capability

## ADDED Requirements

### Requirement: 系统 MUST 支持语音设置倒计时器

用户通过语音指令设置倒计时，系统 SHALL 在指定时间后语音提醒。

#### Scenario: 设置简单倒计时

- Given 用户唤醒 PiBuddy
- When 用户说"设个5分钟倒计时"
- Then 系统创建一个5分钟倒计时
- And 系统回复"已设置5分钟倒计时"
- And 5分钟后系统播报"倒计时结束"

#### Scenario: 设置带标签的倒计时

- Given 用户唤醒 PiBuddy
- When 用户说"帮我倒计时3分钟，提醒我关火"
- Then 系统创建一个3分钟倒计时，标签为"关火"
- And 系统回复"已设置3分钟倒计时，提醒内容：关火"
- And 3分钟后系统播报"关火提醒时间到了"

### Requirement: 系统 MUST 支持查看当前倒计时

用户 SHALL 能够查询正在进行中的倒计时列表和剩余时间。

#### Scenario: 查看倒计时列表

- Given 用户有一个正在进行的倒计时
- When 用户说"有哪些倒计时"
- Then 系统回复当前倒计时列表及剩余时间

#### Scenario: 查询无倒计时

- Given 用户没有正在进行的倒计时
- When 用户说"有哪些倒计时"
- Then 系统回复"当前没有正在进行的倒计时"

### Requirement: 系统 MUST 支持取消倒计时

用户 SHALL 能够取消正在进行的倒计时。

#### Scenario: 取消指定倒计时

- Given 用户有一个ID为"timer_001"的倒计时正在进行
- When 用户说"取消倒计时 timer_001"
- Then 系统取消该倒计时
- And 系统回复"倒计时已取消"
