# 技术设计文档

## 1. 架构概览

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│  PiBuddy    │────▶│  Home Assistant  │────▶│  米家设备   │
│  (语音控制) │     │  + xiaomi_home   │     │             │
└─────────────┘     └──────────────────┘     └─────────────┘
       │                    │
       │   REST API         │   MIoT-Spec-V2
       │   (HTTP)           │   (云端/本地)
       ▼                    ▼
```

## 2. Home Assistant API

### 2.1 认证

使用 Long-Lived Access Token 认证：

```bash
# 创建 Token: Home Assistant → 用户设置 → 安全 → 长期访问令牌
curl -H "Authorization: Bearer $TOKEN" http://localhost:8123/api/
```

### 2.2 核心端点

| 端点 | 说明 |
|------|------|
| `GET /api/states` | 获取所有设备状态 |
| `GET /api/states/<entity_id>` | 获取单个设备状态 |
| `POST /api/services/<domain>/<service>` | 调用服务（控制设备） |

### 2.3 设备控制示例

```bash
# 开灯
POST /api/services/light/turn_on
{"entity_id": "light.xiaomi_light"}

# 设置亮度
POST /api/services/light/turn_on
{"entity_id": "light.xiaomi_light", "brightness_pct": 50}

# 关灯
POST /api/services/light/turn_off
{"entity_id": "light.xiaomi_light"}

# 设置空调温度
POST /api/services/climate/set_temperature
{"entity_id": "climate.xiaomi_ac", "temperature": 26}
```

## 3. 数据结构

### 3.1 配置

```go
type HomeAssistantConfig struct {
    Enabled  bool   `yaml:"enabled"`
    URL      string `yaml:"url"`       // e.g., "http://localhost:8123"
    Token    string `yaml:"token"`     // Long-Lived Access Token
    EntityPrefix string `yaml:"entity_prefix"` // 可选，过滤设备前缀
}
```

### 3.2 设备状态

```go
type DeviceState struct {
    EntityID    string                 `json:"entity_id"`
    State       string                 `json:"state"`      // on/off/...
    Attributes  map[string]interface{} `json:"attributes"` // 亮度、温度等
    LastChanged string                 `json:"last_changed"`
}

// 实体类型
// light.*     - 灯光
// switch.*    - 开关
// climate.*   - 空调
// fan.*       - 风扇
// cover.*     - 窗帘
// sensor.*    - 传感器
// vacuum.*    - 扫地机
```

## 4. 工具定义

### 4.1 控制设备

```json
{
  "name": "ha_control_device",
  "description": "控制智能家居设备。开灯、关灯、调节亮度、设置温度等。",
  "parameters": {
    "type": "object",
    "properties": {
      "entity_id": {
        "type": "string",
        "description": "设备ID，如 light.xiaomi_light"
      },
      "action": {
        "type": "string",
        "enum": ["turn_on", "turn_off", "toggle", "set_brightness", "set_temperature"],
        "description": "操作类型"
      },
      "value": {
        "type": "number",
        "description": "操作值（亮度1-100，温度等）"
      }
    },
    "required": ["entity_id", "action"]
  }
}
```

### 4.2 查询设备状态

```json
{
  "name": "ha_get_device_state",
  "description": "查询智能设备状态。如'灯开着吗'、'空调多少度'等。",
  "parameters": {
    "type": "object",
    "properties": {
      "entity_id": {
        "type": "string",
        "description": "设备ID"
      }
    },
    "required": ["entity_id"]
  }
}
```

### 4.3 列出设备

```json
{
  "name": "ha_list_devices",
  "description": "列出所有可控制的智能家居设备。",
  "parameters": {
    "type": "object",
    "properties": {
      "domain": {
        "type": "string",
        "description": "设备类型过滤：light, switch, climate, fan, cover"
      }
    }
  }
}
```

## 5. 实现细节

### 5.1 HTTP 客户端

```go
type HomeAssistantClient struct {
    baseURL    string
    token      string
    httpClient *http.Client
}

func (c *HomeAssistantClient) CallService(domain, service string, data map[string]interface{}) error
func (c *HomeAssistantClient) GetState(entityID string) (*DeviceState, error)
func (c *HomeAssistantClient) GetStates() ([]DeviceState, error)
```

### 5.2 设备名称映射

LLM 需要将用户友好的设备名称映射到 entity_id：

```
用户说: "打开客厅的灯"
LLM 解析: entity_id = "light.ke_ting_deng" (需要先列出设备获取)
```

### 5.3 错误处理

- 连接失败：提示检查 Home Assistant 是否运行
- 认证失败：提示检查 Token 是否有效
- 设备不存在：列出可用设备供用户选择

## 6. 部署要求

1. **Home Assistant**
   - Core ≥ 2024.11.0
   - 安装 xiaomi_home 集成
   - 创建 Long-Lived Access Token

2. **网络**
   - PiBuddy 和 Home Assistant 在同一网络
   - 或 Home Assistant 有公网访问

3. **米家设备**
   - 设备已在米家 App 中绑定
   - 已通过 xiaomi_home 集成添加到 Home Assistant
