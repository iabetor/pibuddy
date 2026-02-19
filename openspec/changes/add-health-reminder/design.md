# 技术设计文档

## 1. 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                        Pipeline                              │
│  ┌─────────────┐    ┌──────────────┐    ┌───────────────┐  │
│  │ HealthStore │◄───│HealthChecker │───▶│ TTS 播报提醒  │  │
│  │ (配置存储)  │    │ (定时检查)   │    │               │  │
│  └─────────────┘    └──────────────┘    └───────────────┘  │
│         ▲                                        ▲          │
│         │                                        │          │
│  ┌──────┴──────┐                          ┌──────┴──────┐  │
│  │ HealthTool  │                          │ 静音时段检查 │  │
│  │ (工具调用)  │                          │             │  │
│  └─────────────┘                          └─────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## 2. 数据结构

### 2.1 提醒配置

```go
// HealthReminder 健康提醒配置。
type HealthReminder struct {
    Type           ReminderType `json:"type"`            // 提醒类型
    Enabled        bool         `json:"enabled"`         // 是否启用
    IntervalMinutes int         `json:"interval_minutes"` // 间隔（分钟）
    LastRemindedAt time.Time    `json:"last_reminded_at"` // 上次提醒时间
    MedicineName   string       `json:"medicine_name"`    // 药品名称
    Times          []string     `json:"times"`            // 每日提醒时间（medicine专用）
    CreatedAt      time.Time    `json:"created_at"`
}

// ReminderType 提醒类型。
type ReminderType string

const (
    ReminderTypeWater    ReminderType = "water"    // 喝水提醒
    ReminderTypeExercise ReminderType = "exercise" // 久坐提醒
    ReminderTypeMedicine ReminderType = "medicine" // 吃药提醒
)
```

### 2.2 存储格式

存储在 `{data_dir}/health_reminders.json`:

```json
{
  "reminders": {
    "water": {
      "type": "water",
      "enabled": true,
      "interval_minutes": 120,
      "last_reminded_at": "2026-02-19T10:00:00Z"
    },
    "exercise": {
      "type": "exercise",
      "enabled": true,
      "interval_minutes": 60,
      "last_reminded_at": "2026-02-19T10:30:00Z"
    },
    "medicine_blood_pressure": {
      "type": "medicine",
      "enabled": true,
      "medicine_name": "降压药",
      "times": ["08:00", "20:00"],
      "last_reminded_at": "2026-02-19T08:00:00Z"
    }
  },
  "quiet_hours": {
    "start": "23:00",
    "end": "07:00"
  }
}
```

## 3. 工具定义

### 3.1 set_health_reminder

```go
type setHealthReminderArgs struct {
    ReminderType   string   `json:"reminder_type"`    // water/exercise/medicine
    Action         string   `json:"action"`           // start/stop/status
    IntervalMinutes int     `json:"interval_minutes"` // 间隔（分钟）
    MedicineName   string   `json:"medicine_name"`    // 药品名称
    Times          []string `json:"times"`            // 提醒时间列表
}
```

### 3.2 list_health_reminders

列出所有健康提醒状态。

### 3.3 工具返回格式

```go
// 开启提醒
"已开启喝水提醒，每2小时提醒一次"

// 关闭提醒
"喝水提醒已关闭"

// 查询状态
"当前健康提醒状态：\n- 喝水提醒：开启（每2小时）\n- 久坐提醒：开启（每1小时）\n- 降压药：开启（08:00, 20:00）"
```

## 4. 定时检查逻辑

### 4.1 检查间隔

- 喝水/久坐提醒：每 1 分钟检查一次
- 吃药提醒：每 1 分钟检查时间点

### 4.2 检查流程

```
1. 遍历所有已启用的提醒
2. 检查当前时间是否在静音时段
3. 对于间隔型提醒（喝水/久坐）：
   - 计算 elapsed = now - last_reminded_at
   - 如果 elapsed >= interval，触发提醒
4. 对于时间点提醒（吃药）：
   - 检查当前时间是否匹配提醒时间
   - 同一天同一时间点只提醒一次
5. 更新 last_reminded_at
6. TTS 播报提醒
```

### 4.3 静音时段

```go
func (s *HealthStore) isQuietHours() bool {
    now := time.Now()
    currentTime := now.Format("15:04")
    
    // 跨天情况：如 23:00 - 07:00
    if s.config.QuietHoursStart > s.config.QuietHoursEnd {
        return currentTime >= s.config.QuietHoursStart || currentTime < s.config.QuietHoursEnd
    }
    // 同天情况：如 13:00 - 14:00
    return currentTime >= s.config.QuietHoursStart && currentTime < s.config.QuietHoursEnd
}
```

## 5. 提醒文案

### 5.1 喝水提醒

```go
var waterReminders = []string{
    "该喝水了，身体需要水分补充",
    "喝杯水吧，保持健康好习惯",
    "记得喝水哦，多喝水对身体好",
    "休息一下，喝杯水吧",
}
```

### 5.2 久坐提醒

```go
var exerciseReminders = []string{
    "坐太久了，起来活动一下吧",
    "该起来走走了，久坐对身体不好",
    "站起来活动一下，放松放松",
    "休息一下，伸个懒腰吧",
}
```

### 5.3 吃药提醒

```go
func formatMedicineReminder(medicineName string) string {
    templates := []string{
        "该吃%s了，别忘了",
        "提醒你服用%s",
        "%s时间到了",
    }
    return fmt.Sprintf(randomChoice(templates), medicineName)
}
```

## 6. 配置结构

### 6.1 配置文件

```yaml
tools:
  health:
    enabled: true
    water_interval: 120        # 默认喝水间隔（分钟）
    exercise_interval: 60      # 默认久坐间隔（分钟）
    quiet_hours:
      start: "23:00"           # 静音开始时间
      end: "07:00"             # 静音结束时间
```

### 6.2 配置代码

```go
type HealthConfig struct {
    Enabled          bool   `yaml:"enabled"`
    WaterInterval    int    `yaml:"water_interval"`    // 默认 120 分钟
    ExerciseInterval int    `yaml:"exercise_interval"` // 默认 60 分钟
    QuietHours       QuietHoursConfig `yaml:"quiet_hours"`
}

type QuietHoursConfig struct {
    Start string `yaml:"start"` // 静音开始时间，如 "23:00"
    End   string `yaml:"end"`   // 静音结束时间，如 "07:00"
}
```

## 7. 文件结构

```
internal/tools/
├── health.go          # 工具定义和实现
├── health_store.go    # 存储和定时检查
└── health_test.go     # 测试用例
```

## 8. Pipeline 集成

```go
// 在 NewPipeline 中
healthStore, err := tools.NewHealthStore(cfg.Tools.DataDir, tools.HealthConfig{
    WaterInterval:    cfg.Tools.Health.WaterInterval,
    ExerciseInterval: cfg.Tools.Health.ExerciseInterval,
    QuietHoursStart:  cfg.Tools.Health.QuietHours.Start,
    QuietHoursEnd:    cfg.Tools.Health.QuietHours.End,
})
if err != nil {
    return fmt.Errorf("初始化健康提醒存储失败: %w", err)
}
p.healthStore = healthStore

// 注册工具
p.toolRegistry.Register(tools.NewSetHealthReminderTool(healthStore))
p.toolRegistry.Register(tools.NewListHealthRemindersTool(healthStore))

// 启动定时检查
go p.healthReminderChecker(ctx)

// 新增方法
func (p *Pipeline) healthReminderChecker(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            reminders := p.healthStore.CheckAndTrigger()
            for _, r := range reminders {
                p.speakText(ctx, r.Message)
            }
        }
    }
}
```

## 9. 错误处理

| 场景 | 处理方式 |
|------|---------|
| 存储文件损坏 | 删除并重建，返回空状态 |
| 配置为空 | 使用默认值 |
| 时间格式错误 | 忽略该项，记录日志 |
| 静音时段 | 不播报，但更新提醒时间 |

## 10. 测试用例

1. 开启/关闭喝水提醒
2. 开启/关闭久坐提醒
3. 添加/删除吃药提醒
4. 静音时段检测
5. 时间间隔计算
6. 多次提醒随机文案
