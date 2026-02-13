# Tasks: add-function-calling-and-tools

## Phase 1: Function Calling 框架

- [ ] 扩展 `Message` 结构体，增加 `ToolCalls`、`ToolCallID`、`Name` 字段
- [ ] 定义 `Tool` 接口和 `Registry` 工具注册表
- [ ] 修改 `openai.go`，`chatRequest` 支持 `tools` 参数
- [ ] 修改 `openai.go` SSE 解析，支持 `tool_calls` 增量拼接和 `finish_reason=tool_calls`
- [ ] 新增 `ChatWithTools()` 方法（非流式），用于工具调用轮次
- [ ] 修改 `pipeline.processQuery()`，实现工具调用循环（最多 3 轮）
- [ ] 编写 Function Calling 单元测试

## Phase 2: 简单本地工具

- [ ] 实现 `datetime` 工具 — 获取当前日期时间
- [ ] 实现 `calculator` 工具 — 数学表达式求值
- [ ] 编写单元测试

## Phase 3: 天气查询

- [ ] 新增 `ToolsConfig` 配置结构体和 YAML 配置
- [ ] 实现 `weather` 工具 — 和风天气 API 集成
- [ ] 支持实时天气 + 3 日预报
- [ ] 编写单元测试

## Phase 4: 闹钟/提醒

- [ ] 实现闹钟 JSON 存储
- [ ] 实现 `alarm` 工具 — 创建/查询/删除闹钟
- [ ] Pipeline 中启动闹钟检查 goroutine，到时 TTS 播报
- [ ] 编写单元测试

## Phase 5: 备忘录

- [ ] 实现备忘录 JSON 存储
- [ ] 实现 `memo` 工具 — 创建/查询/删除备忘录
- [ ] 编写单元测试

## Phase 6: 新闻 & 股票

- [ ] 实现 `news` 工具 — 热点新闻查询
- [ ] 实现 `stock` 工具 — A 股实时行情查询
- [ ] 编写单元测试

## Phase 7: Prompt 优化 & 集成测试

- [ ] 优化 system_prompt，增加翻译/笑话/故事/百科能力描述
- [ ] 集成测试：端到端验证所有工具
- [ ] 更新配置文件和文档
