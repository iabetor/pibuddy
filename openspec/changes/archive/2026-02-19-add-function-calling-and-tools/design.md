# Design: add-function-calling-and-tools

## 1. Function Calling 架构

### 工具调用流程

```
用户语音 → ASR → 文本
                    ↓
              processQuery()
                    ↓
          LLM ChatStream (带 tools 定义)
                    ↓
            ┌───────┴───────┐
            │               │
      finish_reason     finish_reason
        ="stop"         ="tool_calls"
            │               │
         流式 TTS      执行工具函数
                            ↓
                    tool result → LLM (第二轮)
                            ↓
                      ┌─────┴─────┐
                      │           │
                   ="stop"   ="tool_calls"
                      │        (最多3轮)
                   流式 TTS
```

### 关键决策

**Q: 工具调用时用流式还是非流式？**
- 第一轮请求用**流式**，这样普通对话不受影响
- 如果检测到 `tool_calls`（`finish_reason=tool_calls`），收集完工具调用信息
- 执行工具后，第二轮请求仍用**流式**，最终回复流式输出到 TTS
- 循环最多 3 轮，防止死循环

**Q: 工具定义放在哪？**
- 每个工具实现 `Tool` 接口，自描述（名称、描述、参数 schema）
- `Registry` 统一管理，pipeline 初始化时注册所有工具
- 工具定义在每次 LLM 请求时通过 `tools` 参数传入

**Q: 天气 API 选型？**
- 选择**和风天气（QWeather）**，免费版每天 1000 次
- API 格式简单，支持城市名直接查询
- 备选：心知天气（同样免费 1000 次/天）

**Q: 新闻/股票 API 选型？**
- 新闻：使用韩小韩 API 或天行数据等免费接口获取热搜/头条
- 股票：使用腾讯财经/新浪财经公开接口（无需注册），支持 A 股实时行情

**Q: 闹钟持久化？**
- 使用 JSON 文件存储（`~/.pibuddy/alarms.json`）
- 简单场景不引入数据库
- Pipeline 启动时加载，定时器检查

## 2. 核心接口设计

### Tool 接口

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage  // JSON Schema
    Execute(ctx context.Context, args json.RawMessage) (string, error)
}
```

### Registry

```go
type Registry struct {
    tools map[string]Tool
}

func (r *Registry) Register(t Tool)
func (r *Registry) Get(name string) (Tool, bool)
func (r *Registry) Definitions() []ToolDefinition  // 用于 LLM tools 参数
```

### Message 扩展

```go
type Message struct {
    Role       string     `json:"role"`
    Content    string     `json:"content,omitempty"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
    ToolCallID string     `json:"tool_call_id,omitempty"`
    Name       string     `json:"name,omitempty"`
}

type ToolCall struct {
    ID       string       `json:"id"`
    Type     string       `json:"type"`
    Function FunctionCall `json:"function"`
}

type FunctionCall struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"`
}
```

## 3. 各工具设计

| 工具 | 输入参数 | 输出 | 数据源 |
|---|---|---|---|
| `get_datetime` | 无 | 当前日期、时间、星期 | 本地 |
| `calculate` | `expression: string` | 计算结果 | 本地 |
| `get_weather` | `city: string` | 实时天气+3日预报 | 和风天气 API |
| `set_alarm` | `time: string, message: string` | 确认信息 | 本地 JSON |
| `list_alarms` | 无 | 闹钟列表 | 本地 JSON |
| `delete_alarm` | `id: string` | 确认信息 | 本地 JSON |
| `add_memo` | `content: string` | 确认信息 | 本地 JSON |
| `list_memos` | 无 | 备忘录列表 | 本地 JSON |
| `delete_memo` | `id: string` | 确认信息 | 本地 JSON |
| `get_news` | `category: string (可选)` | 热点新闻列表 | 公开 API |
| `get_stock` | `code: string` | 股价/涨跌幅 | 公开 API |
