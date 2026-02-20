# 米家智能家居接入调研报告

## 1. 背景

PiBuddy 运行在树莓派上，需要调研如何接入米家智能家居设备，实现语音控制智能设备。

---

## 2. 接入方案对比

### 方案一：Home Assistant 集成（推荐）

**2024年12月，小米官方开源了 Home Assistant 集成组件**，这是最推荐的方案。

#### 技术架构

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│  PiBuddy    │────▶│  Home Assistant  │────▶│  米家设备   │
│  (语音控制) │     │  + xiaomi_home   │     │  (云端/本地)│
└─────────────┘     └──────────────────┘     └─────────────┘
```

#### 通信模式

| 模式 | 说明 | 优势 | 限制 |
|------|------|------|------|
| **云端控制** | 通过 MIoT 云 MQTT Broker | 无需额外硬件，全球可用 | 依赖网络 |
| **本地控制** | 通过小米中央网关 | 响应快，离线可用 | 需要网关，仅中国大陆 |
| **LAN 控制** | 局域网直连 WiFi 设备 | 不依赖云端 | 仅支持 WiFi 设备 |

#### 认证方式
- OAuth 2.0 登录流程
- 不保存用户密码
- 支持在米家 App 中撤销授权

#### 核心协议：MIoT-Spec-V2
- 小米 IoT 平台标准化协议
- 设备功能描述：产品 → 设备 → 服务 → 属性/事件/动作
- 自动映射为 Home Assistant 实体

#### 安装要求
- Home Assistant Core ≥ 2024.11.0
- Home Assistant OS ≥ 13.0

#### 安装方式
```bash
# 命令行安装
cd config
git clone https://github.com/XiaoMi/ha_xiaomi_home.git
cd ha_xiaomi_home
./install.sh install
```

或通过 HACS 自定义仓库安装。

---

### 方案二：小米 IoT 开放平台

适用于智能硬件厂商，将设备接入米家 App。

#### 开发方式
- 基于 React Native 开发米家扩展程序
- SDK: miot-plugin-sdk
- 需要 NodeJS ≥ 12.13.1

#### 适用场景
- 硬件厂商开发米家兼容设备
- 不适合第三方控制现有米家设备

---

### 方案三：第三方开源库

#### 1. python-miio
```bash
pip install python-miio
```
- 支持 WiFi 设备控制
- 需要获取设备 token
- 社区维护，设备支持有限

#### 2. hass-xiaomi-miot (Xiaomi Miot Auto)
- 非官方 Home Assistant 集成
- 支持更多设备类型
- 现已被官方方案替代

---

## 3. 推荐实现方案

### 方案架构

```
PiBuddy (树莓派)
    │
    ├── Home Assistant (Docker)
    │       │
    │       └── xiaomi_home 集成
    │               │
    │               └── 米家设备控制
    │
    └── 语音助手服务
            │
            └── 调用 HA REST API 控制设备
```

### 实现步骤

1. **部署 Home Assistant**
   ```bash
   docker run -d \
     --name homeassistant \
     --privileged \
     --restart=unless-stopped \
     -e TZ=Asia/Shanghai \
     -v /path/to/config:/config \
     -v /run/dbus:/run/dbus:ro \
     --network=host \
     ghcr.io/home-assistant/home-assistant:stable
   ```

2. **安装小米集成**
   - HACS 添加自定义仓库
   - 搜索 "Xiaomi Home" 安装
   - 配置 OAuth 登录

3. **PiBuddy 集成**
   - 添加 Home Assistant 工具
   - 通过 REST API 控制设备
   - 或使用 WebSocket 实时获取状态

### 工具定义示例

```go
// HomeAssistantTool 控制米家设备
type HomeAssistantTool struct {
    haURL   string
    haToken string
}

// 支持的操作
// - turn_on_light: 开灯
// - turn_off_light: 关灯
// - set_brightness: 设置亮度
// - set_temperature: 设置温度
// - get_device_state: 查询设备状态
```

### API 调用示例

```bash
# 开灯
curl -X POST \
  -H "Authorization: Bearer $HA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"entity_id": "light.xiaomi_light"}' \
  http://localhost:8123/api/services/light/turn_on

# 设置亮度
curl -X POST \
  -H "Authorization: Bearer $HA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"entity_id": "light.xiaomi_light", "brightness_pct": 50}' \
  http://localhost:8123/api/services/light/turn_on

# 查询设备状态
curl -X GET \
  -H "Authorization: Bearer $HA_TOKEN" \
  http://localhost:8123/api/states/light.xiaomi_light
```

---

## 4. 支持的设备类型

根据 MIoT-Spec-V2 协议，支持以下类型：

| 类型 | 实体映射 | 示例设备 |
|------|----------|----------|
| 灯光 | light | 米家台灯、智能灯泡 |
| 开关 | switch | 智能插座、墙壁开关 |
| 传感器 | sensor | 温湿度、人体感应、门窗传感器 |
| 空调 | climate | 米家空调、空调伴侣 |
| 风扇 | fan | 米家风扇、空气净化器 |
| 扫地机 | vacuum | 米家扫地机器人 |
| 窗帘 | cover | 米家智能窗帘 |
| 摄像头 | camera | 小米摄像头 |
| 门锁 | lock | 米家智能门锁 |
| 音箱 | media_player | 小爱音箱 |

---

## 5. 实现优先级

### 第一阶段：基础控制
1. 开关类设备（灯、插座）
2. 调节类设备（亮度、温度）
3. 状态查询

### 第二阶段：高级功能
1. 传感器数据读取
2. 场景联动
3. 自动化触发

### 第三阶段：完整集成
1. 设备自动发现
2. 多房间支持
3. 设备分组管理

---

## 6. 配置示例

```yaml
# configs/pibuddy.yaml 新增配置
tools:
  home_assistant:
    enabled: true
    url: "http://localhost:8123"
    token: "${PIBUDDY_HA_TOKEN}"
    # 可选：指定控制的设备前缀
    entity_prefix: "xiaomi_"
```

---

## 7. 风险与注意事项

1. **网络依赖**：云端模式需要稳定网络
2. **认证安全**：Token 需妥善保管
3. **设备兼容**：部分设备可能需要本地网关
4. **地区限制**：本地控制仅中国大陆可用
5. **并发控制**：避免频繁调用 API 触发限流

---

## 8. 参考资料

- [小米官方 Home Assistant 集成](https://github.com/XiaoMi/ha_xiaomi_home)
- [Home Assistant 官方文档](https://www.home-assistant.io/)
- [小米 IoT 开发者平台](https://iot.mi.com/)
- [MIoT-Spec-V2 协议规范](https://iot.mi.com/v2/new/doc/plugin/quickstart/quick-start)
