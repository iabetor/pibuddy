## ADDED Requirements

### Requirement: 本地故事库

系统 SHALL 内置本地故事库，支持离线播放经典故事。

#### Scenario: 加载本地故事库
- **WHEN** 系统启动时
- **THEN** 从 `models/stories/` 目录加载所有故事文件
- **AND** 构建故事索引供快速查询

#### Scenario: 本地故事匹配
- **WHEN** 用户请求讲故事
- **AND** 提供了关键词或分类
- **THEN** 系统在本地库中匹配故事
- **AND** 返回最匹配的故事内容

#### Scenario: 本地库未命中
- **WHEN** 用户请求讲故事
- **AND** 本地库中没有匹配的故事
- **THEN** 系统尝试从外部 API 获取

### Requirement: 外部故事 API

系统 SHALL 支持从外部故事 API 获取故事内容。

#### Scenario: API 获取故事成功
- **WHEN** 本地库未命中
- **AND** 外部 API 可用
- **THEN** 调用故事 API 获取故事
- **AND** 缓存获取的故事内容

#### Scenario: API 缓存命中
- **WHEN** 用户请求的故事已缓存
- **THEN** 直接返回缓存内容
- **AND** 不调用外部 API

#### Scenario: API 不可用
- **WHEN** 外部 API 不可用或超时
- **THEN** 降级到 LLM 生成或返回错误提示

### Requirement: LLM 故事生成

系统 SHALL 在必要时使用 LLM 生成定制化故事。

#### Scenario: LLM 生成定制故事
- **WHEN** 用户要求定制化故事（如"讲一个关于小猫的故事"）
- **AND** 本地库和 API 都没有匹配
- **THEN** 调用 LLM 生成故事
- **AND** 缓存生成的故事

#### Scenario: 限制生成长度
- **WHEN** LLM 生成故事
- **THEN** 限制 max_tokens 为 800
- **AND** 确保故事长度适中

### Requirement: 故事来源优先级

系统 SHALL 按优先级选择故事来源。

#### Scenario: 优先级顺序
- **WHEN** 用户请求讲故事
- **THEN** 按以下顺序尝试：
  1. 本地故事库
  2. 外部故事 API
  3. LLM 生成

#### Scenario: 记录来源日志
- **WHEN** 返回故事内容
- **THEN** 记录故事来源（local/api/llm）
- **AND** 用于统计和优化
