# Tasks: 扩展天气 API

## 1. 重构城市搜索，缓存经纬度

- [x] 1.1 新增 `cityInfo` 结构体，包含 `id`, `name`, `latitude`, `longitude`
- [x] 1.2 修改 `lookupCity` 返回 `*cityInfo` 而非 `(string, string, error)`
- [x] 1.3 解析 Geo API 响应中的 `lat` 和 `lon` 字段

## 2. 扩展天气预报支持 7/15 天

- [x] 2.1 修改 `weatherArgs` 添加 `Days int` 字段
- [x] 2.2 修改 `Parameters()` 添加 days 参数描述
- [x] 2.3 修改 `getForecast` 根据 days 值构建 API 路径（`3d`, `7d`, `15d`）
- [x] 2.4 更新 `Description()` 说明支持 3/7/15 天

## 3. 新增空气质量查询工具

- [x] 3.1 创建 `AirQualityTool` 结构体（复用 WeatherTool 的认证逻辑）
- [x] 3.2 实现 `Name()`, `Description()`, `Parameters()` 方法
- [x] 3.3 实现 `Execute()` 方法：
  - 调用 `lookupCity` 获取经纬度
  - 调用 `/airquality/v1/current/{lat}/{lon}`
  - 解析返回数据，格式化输出
- [x] 3.4 定义响应结构体 `qweatherAirQualityResp`

## 4. 注册新工具

- [x] 4.1 在 `internal/pipeline/pipeline.go` 中注册 `AirQualityTool`

## 5. 更新测试

- [x] 5.1 现有测试通过

## 6. 清理测试代码

- [x] 6.1 测试文件已清理

## 7. 编译验证

- [x] 7.1 `go build ./...` 通过
- [x] 7.2 `go test ./...` 通过
