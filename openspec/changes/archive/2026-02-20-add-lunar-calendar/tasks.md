# Tasks: 添加农历查询功能

## 1. 添加依赖

- [x] 1.1 添加 `github.com/6tail/lunar-go` 依赖到 `go.mod`
- [x] 1.2 运行 `go mod tidy` 确保依赖正确下载

## 2. 实现工具

- [x] 2.1 创建 `internal/tools/lunar.go`
- [x] 2.2 定义 `LunarDateTool` 结构体
- [x] 2.3 实现 `Name()`, `Description()`, `Parameters()` 方法
- [x] 2.4 实现 `Execute()` 方法：
  - 解析参数（`include_huangli`）
  - 获取当前时间并转换为农历
  - 提取农历日期、干支、生肖信息
  - 提取节气、节日信息
  - 可选提取黄历信息（宜忌、冲煞）
  - 构建并返回 JSON 结果
- [x] 2.5 添加错误处理和日志记录

## 3. 注册工具

- [x] 3.1 在 `internal/pipeline/pipeline.go` 的 `initTools()` 方法中注册 `LunarDateTool`

## 4. 编写测试

- [x] 4.1 创建 `internal/tools/lunar_test.go`
- [x] 4.2 测试工具元数据（Name、Description、Parameters）
- [x] 4.3 测试基本农历查询（无黄历）
- [x] 4.4 测试农历查询（包含黄历）
- [x] 4.5 测试干支、生肖计算准确性
- [x] 4.6 测试节气、节日识别

## 5. 验证和清理

- [x] 5.1 运行 `go build ./...` 确保编译通过
- [x] 5.2 运行 `go test ./...` 确保所有测试通过
- [ ] 5.3 手动测试：启动 pibuddy，询问"今天农历几号"

## 6. 文档更新（可选）

- [ ] 6.1 更新 README，添加农历查询功能说明
- [ ] 6.2 添加使用示例到文档

## 预计工作量

- 开发时间：2-3 小时
- 测试时间：1 小时
- 总计：3-4 小时
