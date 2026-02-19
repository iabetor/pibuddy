# Proposal: add-function-calling-and-tools

## Why

PiBuddy 当前仅支持纯文本对话，LLM 无法获取实时信息（天气、时间、新闻、股票等），
也无法执行本地操作（闹钟、备忘录、计算器）。用户提问"武汉天气怎么样"时只能回复
"无法查询"。需要实现 Function Calling 框架及一系列实用工具，向小爱同学的能力对齐。

## What Changes

### 1. Function Calling 框架 (核心基础)

- **MODIFIED** `internal/llm/provider.go` — `Message` 结构体扩展，支持 `tool_calls`、`tool_call_id`、`role=tool`
- **MODIFIED** `internal/llm/openai.go` — `chatRequest` 增加 `tools` 字段；SSE 解析支持 `tool_calls` 增量拼接；非流式模式处理工具调用
- **ADDED** `internal/tools/registry.go` — 工具注册表，管理所有可用工具的定义和执行函数
- **ADDED** `internal/tools/tool.go` — `Tool` 接口定义（Name, Description, Parameters, Execute）
- **MODIFIED** `internal/pipeline/pipeline.go` — `processQuery` 增加工具调用循环：LLM 返回 tool_calls → 执行工具 → 结果回传 LLM → 生成最终回复

### 2. 天气查询工具

- **ADDED** `internal/tools/weather.go` — 调用和风天气 API（QWeather），支持按城市名查询实时天气和 3 日预报
- **MODIFIED** `internal/config/config.go` — 新增 `ToolsConfig` 配置，包含 `WeatherAPIKey`
- **MODIFIED** `configs/pibuddy.yaml` — 新增 `tools.weather_api_key` 配置项

### 3. 日期时间工具

- **ADDED** `internal/tools/datetime.go` — 返回当前日期、时间、星期、农历（可选）

### 4. 计算器工具

- **ADDED** `internal/tools/calculator.go` — 支持四则运算、百分比、幂运算等数学表达式求值

### 5. 闹钟/提醒工具

- **ADDED** `internal/tools/alarm.go` — 设置定时提醒，到时通过 TTS 播报提醒内容
- **ADDED** `internal/tools/alarm_store.go` — 闹钟持久化存储（JSON 文件）
- **MODIFIED** `internal/pipeline/pipeline.go` — 启动闹钟检查 goroutine

### 6. 备忘录工具

- **ADDED** `internal/tools/memo.go` — 创建、查询、删除备忘录
- **ADDED** `internal/tools/memo_store.go` — 备忘录持久化存储（JSON 文件）

### 7. 新闻播报工具

- **ADDED** `internal/tools/news.go` — 调用免费新闻 API 获取热点新闻摘要

### 8. 股票行情工具

- **ADDED** `internal/tools/stock.go` — 查询 A 股实时行情（股价、涨跌幅）

### 9. 中英翻译（Prompt 优化，无需工具）

- **MODIFIED** `configs/pibuddy.yaml` — 在 system_prompt 中增加翻译能力描述

### 10. 笑话/故事/百科（Prompt 优化，无需工具）

- **MODIFIED** `configs/pibuddy.yaml` — 在 system_prompt 中增加讲故事、讲笑话的能力描述

## Impact

### Specs affected
- `openspec/specs/pipeline.md` — processQuery 流程变更，新增工具调用循环
- `openspec/specs/llm.md` — Provider 接口和 Message 结构体变更
- NEW `openspec/specs/tools.md` — 工具框架和各工具的规格定义

### Code files affected
- `internal/llm/provider.go` — Message 扩展
- `internal/llm/openai.go` — Function Calling 支持
- `internal/pipeline/pipeline.go` — 工具调用循环 + 闹钟 goroutine
- `internal/config/config.go` — ToolsConfig
- `configs/pibuddy.yaml` — 工具配置
- NEW `internal/tools/` — 整个工具包（8+ 文件）

### External dependencies
- **和风天气 API** — 免费版每天 1000 次调用，需用户注册获取 API Key（https://dev.qweather.com/）
- **新闻 API** — 使用免费公开接口
- **股票 API** — 使用免费公开接口
- `github.com/Knetic/govaluate` 或手写表达式求值 — 计算器功能

### User actions required
- 注册和风天气开发者账号，获取免费 API Key
- 配置 `PIBUDDY_WEATHER_API_KEY` 环境变量
