# Proposal: 扩展天气 API 支持 7/15 天预报和空气质量查询

## Motivation

当前 `get_weather` 工具仅支持 3 天天气预报，用户询问"未来一周天气"或"半个月天气"时无法满足需求。同时，用户也无法查询空气质量信息。

经测试验证，当前和风天气订阅支持：
- ✅ 3天/7天/15天天气预报
- ✅ 实时空气质量
- ✅ 空气质量每日预报

## Proposal

### 1. 扩展天气预报天数

修改 `get_weather` 工具，新增 `days` 参数：

```json
{
  "type": "object",
  "properties": {
    "city": { "type": "string", "description": "城市名称，例如 北京、上海、武汉" },
    "days": { 
      "type": "integer", 
      "description": "预报天数，可选值：3、7、15，默认为 3",
      "enum": [3, 7, 15]
    }
  },
  "required": ["city"]
}
```

LLM 根据用户问题自动选择天数：
- "明天天气" → days=3
- "未来一周天气" → days=7
- "半个月天气" → days=15

### 2. 新增空气质量查询工具

新增 `get_air_quality` 工具：

```json
{
  "type": "object",
  "properties": {
    "city": { "type": "string", "description": "城市名称" }
  },
  "required": ["city"]
}
```

返回：AQI 指数、等级、主要污染物、健康建议等。

### 3. 缓存城市经纬度

空气质量 API 需要经纬度作为路径参数：`/airquality/v1/current/{lat}/{lon}`

修改 `lookupCity` 方法，返回城市信息时包含经纬度：

```go
type CityInfo struct {
    ID        string  // LocationID (如 101010100)
    Name      string  // 城市名称
    Latitude  string  // 纬度 (如 39.90)
    Longitude string  // 经度 (如 116.40)
}
```

## Impact

- `internal/tools/weather.go` — 添加 days 参数、缓存经纬度、新增空气质量查询
- `internal/tools/weather_test.go` — 更新测试用例

## API 参考

### 天气预报
- 路径：`/v7/weather/{days}?location={locationID}`
- days 可选值：`3d`, `7d`, `15d`

### 空气质量（实时）
- 路径：`/airquality/v1/current/{lat}/{lon}`
- 注意：**纬度在前，经度在后**

### 空气质量（每日预报）
- 路径：`/airquality/v1/daily/{lat}/{lon}`
- 返回未来 3 天空气质量预报

### 城市搜索
- 路径：`/geo/v2/city/lookup?location={cityName}`
- 返回：`id`, `lat`, `lon` 等字段
