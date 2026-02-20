# xiaomi-smart-home Specification

## Purpose
TBD - created by archiving change add-xiaomi-smart-home. Update Purpose after archive.
## Requirements
### Requirement: 系统 MUST 支持通过语音控制小米智能家居设备

用户 SHALL 能够通过语音指令控制已连接的米家智能设备。

#### Scenario: 开灯

- Given 用户已配置 Home Assistant 并添加了米家设备
- When 用户说"打开客厅的灯"
- Then 系统调用 Home Assistant API 开灯
- And 系统回复"客厅的灯已打开"

#### Scenario: 关灯

- Given 客厅的灯是开着的
- When 用户说"关掉客厅的灯"
- Then 系统调用 Home Assistant API 关灯
- And 系统回复"客厅的灯已关闭"

#### Scenario: 调节亮度

- Given 用户已配置可调光的智能灯
- When 用户说"把卧室灯亮度调到50%"
- Then 系统调用 Home Assistant API 设置亮度
- And 系统回复"卧室灯亮度已设为50%"

#### Scenario: 设置空调温度

- Given 用户已配置小米空调
- When 用户说"把空调温度设为26度"
- Then 系统调用 Home Assistant API 设置温度
- And 系统回复"空调温度已设为26度"

### Requirement: 系统 MUST 支持查询智能设备状态

用户 SHALL 能够查询智能设备的当前状态。

#### Scenario: 查询灯光状态

- Given 客厅的灯是开着的
- When 用户说"客厅的灯开着吗"
- Then 系统回复"客厅的灯是开着的"

#### Scenario: 查询空调状态

- Given 空调温度为26度
- When 用户说"空调多少度"
- Then 系统回复"空调当前温度是26度"

### Requirement: 系统 MUST 支持列出可用设备

用户 SHALL 能够查询可控制的设备列表。

#### Scenario: 列出所有设备

- Given 用户已添加多个米家设备
- When 用户说"有哪些智能设备"
- Then 系统列出所有可控制的设备

#### Scenario: 按类型筛选设备

- Given 用户已添加灯光、开关等设备
- When 用户说"有哪些灯"
- Then 系统列出所有灯光设备

