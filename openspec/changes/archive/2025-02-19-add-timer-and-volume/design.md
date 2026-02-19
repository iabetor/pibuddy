# 技术设计文档

## 1. 倒计时器设计

### 1.1 数据结构

```go
// TimerEntry 倒计时条目
type TimerEntry struct {
    ID        string    `json:"id"`         // 唯一标识
    Duration   int       `json:"duration"`   // 总时长（秒）
    Remaining int       `json:"remaining"`  // 剩余时间（秒）
    Label     string    `json:"label"`      // 标签/提醒内容
    StartTime time.Time `json:"start_time"` // 开始时间
    ExpiresAt time.Time `json:"expires_at"` // 到期时间
}

// TimerStore 倒计时存储
type TimerStore struct {
    mu       sync.RWMutex
    filePath string
    timers   map[string]*TimerEntry
    timerMap map[string]*time.Timer // Go timer 引用
    onExpire func(entry TimerEntry) // 到期回调
}
```

### 1.2 时间解析策略

支持多种自然语言输入：

| 输入 | 解析结果 |
|------|----------|
| "5分钟" | 300秒 |
| "30秒" | 30秒 |
| "1分30秒" | 90秒 |
| "一个半小时" | 5400秒 |
| "两刻钟" | 1800秒 |

LLM 负责将自然语言转换为秒数，工具接收 `duration_seconds` 参数。

### 1.3 到期通知机制

```
┌─────────────┐     ┌──────────────┐     ┌────────────┐
│ SetTimerTool│────▶│ TimerStore   │────▶│ Go Timer   │
└─────────────┘     └──────────────┘     └────────────┘
                           │
                           │ onExpire callback
                           ▼
                    ┌──────────────┐
                    │ Pipeline     │
                    │ (播放提醒)   │
                    └──────────────┘
```

### 1.4 并发安全

- 使用 `sync.RWMutex` 保护 timers map
- 倒计时到期回调在独立 goroutine 执行
- 通过 channel 与 pipeline 通信，避免直接调用

---

## 2. 音量控制设计

### 2.1 平台抽象

```go
// VolumeController 音量控制器接口
type VolumeController interface {
    GetVolume() (int, error)           // 获取当前音量 (0-100)
    SetVolume(volume int) error        // 设置音量 (0-100)
    AdjustVolume(delta int) error      // 相对调节 (-100 ~ +100)
    IsMuted() (bool, error)            // 是否静音
    SetMute(muted bool) error          // 设置静音
}
```

### 2.2 Linux 实现

**优先 PulseAudio**（现代 Linux 默认）：

```bash
# 获取音量
pactl get-sink-volume @DEFAULT_SINK@ | grep -oP '\d+(?=%)' | head -1

# 设置音量
pactl set-sink-volume @DEFAULT_SINK@ 50%

# 静音
pactl set-sink-mute @DEFAULT_SINK@ 1
```

**回退 ALSA**：

```bash
# 获取音量
amixer get Master | grep -oP '\d+(?=%)' | head -1

# 设置音量
amixer set Master 50%

# 静音
amixer set Master mute
```

### 2.3 macOS 实现（开发测试）

```bash
# 获取音量
osascript -e 'output volume of (get volume settings)'

# 设置音量
osascript -e 'set volume output volume 50'

# 静音
osascript -e 'set volume output volume 0'
```

### 2.4 错误处理

- 命令执行失败时返回友好错误信息
- 自动检测可用音量控制系统（PulseAudio > ALSA）
- 缓存检测结果，避免重复检测

---

## 3. 工具注册

### 3.1 倒计时器工具定义

```json
{
  "name": "set_timer",
  "description": "设置倒计时器。当用户说'设个倒计时'、'提醒我'等时使用。",
  "parameters": {
    "type": "object",
    "properties": {
      "duration_seconds": {
        "type": "integer",
        "description": "倒计时长（秒）"
      },
      "label": {
        "type": "string",
        "description": "提醒标签，如'关火'、'休息'"
      }
    },
    "required": ["duration_seconds"]
  }
}
```

### 3.2 音量控制工具定义

```json
{
  "name": "set_volume",
  "description": "设置播放音量。当用户说'音量设为X'、'调大音量'时使用。",
  "parameters": {
    "type": "object",
    "properties": {
      "volume": {
        "type": "integer",
        "description": "音量值 (0-100)，-1表示静音切换"
      },
      "relative": {
        "type": "boolean",
        "description": "是否相对调节（true时volume为增量）"
      }
    },
    "required": ["volume"]
  }
}
```

---

## 4. 配置扩展

```yaml
# 可选配置项
volume:
  default: 70        # 默认音量
  step: 10           # 相对调节步长

timer:
  max_concurrent: 5  # 最大同时运行的倒计时数
```
