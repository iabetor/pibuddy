# Design: 扩展天气 API

## 设计决策

### Decision 1: 天气预报使用可选 days 参数

**Why**: 保持向后兼容，现有调用 `get_weather({"city": "北京"})` 仍返回 3 天预报。LLM 可根据用户问题自动选择合适的天数。

**Implementation**: 
- 参数类型 `integer`，可选值 `3, 7, 15`
- 默认值为 3
- API 路径从 `/v7/weather/3d` 改为 `/v7/weather/{days}d`

### Decision 2: 空气质量作为独立工具

**Why**: 空气质量查询是独立需求，与天气查询解耦更清晰。用户可能只想查空气质量，不想查天气。

**Implementation**:
- 新增 `get_air_quality` 工具
- 复用城市搜索逻辑获取经纬度
- 调用 `/airquality/v1/current/{lat}/{lon}`

### Decision 3: 缓存城市信息（含经纬度）

**Why**: 
1. 天气 API 使用 LocationID
2. 空气质量 API 需要经纬度
3. 避免重复查询城市信息

**Implementation**:
```go
type cityInfo struct {
    id        string // LocationID
    name      string // 显示名称
    latitude  string // 纬度
    longitude string // 经度
}

func (t *WeatherTool) lookupCity(ctx context.Context, city string) (*cityInfo, error) {
    // 返回完整城市信息
}
```

## 数据流

### 天气查询
```
用户: "北京未来一周天气"
    ↓
LLM 调用: get_weather({"city": "北京", "days": 7})
    ↓
lookupCity("北京") → {id: "101010100", lat: "39.90", lon: "116.40"}
    ↓
GET /v7/weather/7d?location=101010100
    ↓
返回 7 天预报
```

### 空气质量查询
```
用户: "北京空气质量怎么样"
    ↓
LLM 调用: get_air_quality({"city": "北京"})
    ↓
lookupCity("北京") → {id: "101010100", lat: "39.90", lon: "116.40"}
    ↓
GET /airquality/v1/current/39.90/116.40
    ↓
返回 AQI、等级、主要污染物
```

## 返回数据格式

### 天气预报（保持现有格式）
```
北京天气:
实时: 晴, 温度5°C, 体感1°C, 北风3级, 湿度30%
预报:
2026-02-13: 晴转多云, -2~8°C, 北风3-4级
2026-02-14: 多云转阴, 0~10°C, 南风2-3级
...（共 7 或 15 天）
```

### 空气质量（新）
```
北京空气质量:
AQI: 85, 等级: 良
主要污染物: PM2.5
健康建议: 空气质量可接受，但某些污染物可能对极少数异常敏感人群健康有较弱影响。
```
