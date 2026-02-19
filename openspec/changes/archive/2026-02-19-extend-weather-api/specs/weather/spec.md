# Weather Tool Specification

## ADDED Requirements

### Requirement: 天气预报支持多天数

`get_weather` 工具 SHALL 支持查询 3天、7天、15天天气预报，通过 `days` 参数指定。

#### Scenario: 查询 7 天天气预报

- **GIVEN** 用户询问"北京未来一周天气"
- **WHEN** LLM 调用 `get_weather({"city": "北京", "days": 7})`
- **THEN** 系统返回北京未来 7 天天气预报

#### Scenario: 查询 15 天天气预报

- **GIVEN** 用户询问"上海半个月天气"
- **WHEN** LLM 调用 `get_weather({"city": "上海", "days": 15})`
- **THEN** 系统返回上海未来 15 天天气预报

#### Scenario: 默认 3 天预报

- **GIVEN** 用户询问"深圳天气"
- **WHEN** LLM 调用 `get_weather({"city": "深圳"})` 或 `get_weather({"city": "深圳", "days": 3})`
- **THEN** 系统返回深圳未来 3 天天气预报

### Requirement: 空气质量查询

新增 `get_air_quality` 工具，SHALL 支持查询指定城市的实时空气质量。

#### Scenario: 查询空气质量

- **GIVEN** 用户询问"北京空气质量怎么样"
- **WHEN** LLM 调用 `get_air_quality({"city": "北京"})`
- **THEN** 系统返回北京实时空气质量，包括：
  - AQI 数值
  - 空气质量等级（优/良/轻度污染等）
  - 主要污染物
  - 健康建议

### Requirement: 城市信息缓存经纬度

城市搜索功能 SHALL 返回城市信息（含经纬度），供空气质量 API 使用。

#### Scenario: 获取城市经纬度

- **GIVEN** 用户查询某城市的天气或空气质量
- **WHEN** 系统调用 Geo API 搜索城市
- **THEN** 系统缓存城市信息，包括：
  - LocationID
  - 城市名称
  - 纬度 (lat)
  - 经度 (lon)
