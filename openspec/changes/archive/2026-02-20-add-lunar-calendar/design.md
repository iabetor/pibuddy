# Design: 农历查询功能

## Context

PiBuddy 是一个语音助手，用户可以通过自然语言查询日期时间信息。目前系统只能查询公历日期（`get_datetime` 工具），无法查询农历相关内容。农历在中国文化中仍然广泛应用，很多用户的生日、纪念日、传统节日都使用农历。

**约束条件：**
- 必须支持公历农历互转
- 必须支持干支、生肖、节气、节日等传统信息
- 不能依赖外部 API（避免网络延迟和可用性问题）
- 查询速度要快（毫秒级）

## Goals / Non-Goals

**Goals:**
- 支持农历日期查询（年月日、干支、生肖）
- 支持节气查询
- 支持传统节日和法定节假日查询
- 支持黄历查询（宜忌、冲煞等）
- 无外部依赖，纯本地计算

**Non-Goals:**
- 不支持农历转公历的复杂计算（用户输入农历日期转为公历）
- 不支持自定义黄历算法
- 不支持历史或未来超过 2100 年的日期

## Decisions

### Decision 1: 使用 `github.com/6tail/lunar-go` 库

**Why:**
- 成熟稳定，GitHub 1.7k+ stars
- 无第三方依赖，纯 Go 实现
- 功能完整：农历、干支、生肖、节气、节日、黄历等
- MIT 协议，商业友好
- 支持范围 1900-2100 年，满足日常需求
- API 简洁易用

**Alternatives:**
1. **自研算法**
   - 农历算法复杂，需要维护大量数据表
   - 容易出错，维护成本高
   - 开发时间长（需要数天）

2. **其他 Go 农历库**
   - 功能不如 lunar-go 完整
   - 社区活跃度低

### Decision 2: 创建独立的 `LunarDateTool` 工具

**Why:**
- 职责分离清晰
- 与 `DateTimeTool` 互补，不冲突
- 参数设计简单，易于 LLM 理解和调用

**API 设计:**
```go
type LunarDateTool struct{}

func (t *LunarDateTool) Name() string { 
    return "get_lunar_date" 
}

func (t *LunarDateTool) Description() string {
    return "查询农历日期和传统历法信息。当用户询问农历日期、干支纪年、生肖、节气、传统节日、黄历宜忌等问题时使用。"
}

func (t *LunarDateTool) Parameters() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "include_huangli": {
                "type": "boolean",
                "description": "是否包含黄历信息（宜忌、冲煞等），默认false"
            }
        },
        "required": []
    }`)
}
```

### Decision 3: 返回结构化信息

**Why:**
- 便于 LLM 理解和生成自然语言回答
- 便于扩展新字段

**返回格式:**
```json
{
    "solar_date": "2026-02-15",
    "lunar_date": "二〇二六年腊月廿八",
    "year_ganzhi": "丙寅",
    "zodiac": "虎",
    "month_ganzhi": "庚寅",
    "day_ganzhi": "癸卯",
    "constellation": "水瓶座",
    "solar_term": "立春后第11天",
    "next_solar_term": "雨水 (2天后)",
    "festivals": ["春节 (2天后)"],
    "huangli": {
        "yi": ["祭祀", "祈福", "求嗣"],
        "ji": ["动土", "破土"],
        "chong": "冲鸡",
        "sha": "煞西"
    }
}
```

### Decision 4: 分级信息返回

**Why:**
- 默认返回基本信息（农历、干支、生肖、节气）
- 可选返回详细黄历信息（宜忌、冲煞）
- 避免信息过载，提高响应速度

**Implementation:**
```go
func (t *LunarDateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    // 解析参数
    var params struct {
        IncludeHuangli bool `json:"include_huangli"`
    }
    json.Unmarshal(args, &params)
    
    // 获取当前农历信息
    now := time.Now()
    lunar := calendar.NewLunarFromSolar(calendar.NewSolarFromDate(now))
    
    // 构建返回结果
    result := buildLunarResult(lunar, params.IncludeHuangli)
    
    return json.Marshal(result)
}
```

## Risks / Trade-offs

### Risk 1: 依赖库维护风险
- **风险**：lunar-go 库可能停止维护
- **缓解**：
  - 库已稳定，核心算法不太可能变化
  - 可以 fork 仓库自己维护
  - 农历算法相对稳定，未来变化可能性小

### Risk 2: 准确性风险
- **风险**：农历转换可能存在误差
- **缓解**：
  - lunar-go 基于 ChineseLunisolarCalendar 验证
  - 支持 1900-2100 年，覆盖日常需求
  - 提供用户反馈机制，发现问题及时修正

### Risk 3: 信息过载
- **风险**：返回信息太多，影响用户体验
- **缓解**：
  - 默认只返回基本信息
  - 通过参数控制是否返回黄历详情
  - LLM 可以根据用户问题提取关键信息

## Implementation Plan

### Phase 1: 核心功能
1. 添加 `github.com/6tail/lunar-go` 依赖
2. 创建 `internal/tools/lunar.go`，实现 `LunarDateTool`
3. 在 pipeline 中注册工具
4. 编写单元测试

### Phase 2: 优化（可选）
1. 缓存农历计算结果（当天）
2. 添加农历日期转公历功能（如果用户需要）
3. 支持查询历史或未来日期

## Testing Strategy

1. **单元测试**
   - 测试公历转农历
   - 测试干支、生肖计算
   - 测试节气、节日识别
   - 测试黄历信息提取

2. **集成测试**
   - 测试工具注册和调用
   - 测试与 LLM 的交互

3. **边界测试**
   - 测试 1900 年和 2100 年边界
   - 测试闰月情况

## Open Questions

- 是否需要支持用户输入农历日期转为公历？（可以后续独立需求处理）
- 是否需要支持查询历史或未来日期？（当前只支持查询当天）
- 黄历信息是否需要简化？（可以先实现完整版本，根据反馈调整）
