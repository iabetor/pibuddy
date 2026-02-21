# Proposal: 故事功能优化（降低 Token 消耗）

## 变更概述

优化讲故事功能，从纯 LLM 生成改为"本地故事库 + API + LLM 兜底"的混合方案，并在原文朗读模式下完全绕过 LLM，实现零 token 消耗。

## 动机

- **Token 消耗大**：当前通过 LLM 生成故事，一个完整故事消耗 500-2000 tokens
- **成本高**：频繁使用讲故事功能会导致 API 费用快速增加
- **响应慢**：LLM 生成长文本需要较长时间
- **质量不稳定**：每次生成的内容不一致

## 详细设计

### 方案：混合故事来源

```
用户请求讲故事
       ↓
┌──────────────────┐
│  1. 本地故事库   │ ← 优先：经典童话、寓言、成语故事
└────────┬─────────┘
         ↓ 未命中
┌──────────────────┐
│  2. 故事 API     │ ← 扩展：mxnzp 故事大全 API
└────────┬─────────┘
         ↓ 未命中/定制需求
┌──────────────────┐
│  3. LLM 生成     │ ← 兜底：定制化故事
└──────────────────┘
```

### 输出模式与 LLM 绕过机制

**核心优化**：根据 `output_mode` 配置决定是否经过 LLM 处理。

```
┌─────────────────────────────────────────────────────────────┐
│                      output_mode 配置                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  output_mode: "raw"（原文朗读）                              │
│  ┌──────────┐     ┌──────────┐     ┌──────────┐            │
│  │ 工具执行  │ ──→ │ 绕过 LLM │ ──→ │   TTS    │  ✓ 零token │
│  └──────────┘     └──────────┘     └──────────┘            │
│                                                             │
│  output_mode: "summarize"（总结朗读）                        │
│  ┌──────────┐     ┌──────────┐     ┌──────────┐            │
│  │ 工具执行  │ ──→ │   LLM    │ ──→ │   TTS    │  消耗token │
│  └──────────┘     └──────────┘     └──────────┘            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**技术实现**：扩展工具返回结构，支持标记是否跳过 LLM。

```go
// ToolResult 工具执行结果
type ToolResult struct {
    Content  string `json:"content"`   // 返回内容
    SkipLLM  bool   `json:"skip_llm"` // 是否跳过 LLM 直接输出
}
```

当 `SkipLLM=true` 时，Pipeline 检测后直接将 `Content` 送入 TTS，完全跳过 LLM 调用。

### 本地故事库

内置经典故事，分类存储：

```
models/stories/
├── fairy_tales.json      # 童话故事（格林、安徒生）
├── fables.json           # 寓言故事（伊索寓言、中国寓言）
├── chinese_stories.json  # 中国传统故事
├── idiom_stories.json    # 成语故事
└── bedtime_stories.json  # 睡前故事
```

每个文件格式：
```json
[
  {
    "id": "001",
    "title": "小马过河",
    "category": "寓言故事",
    "tags": ["动物", "勇气", "尝试"],
    "content": "从前有一匹小马...",
    "word_count": 500
  }
]
```

### 故事 API 集成

**mxnzp 故事大全 API**（免费）

| 接口 | 说明 |
|------|------|
| `GET /api/story/types` | 获取故事分类 |
| `GET /api/story/list?type_id=1&page=1` | 获取分类下的故事列表 |
| `GET /api/story/search?keyword=xxx` | 搜索故事 |
| `GET /api/story/details?story_id=xxx` | 获取故事详情 |

分类包括：睡前故事、儿童故事、格林童话、安徒生童话、民间故事、益智故事、历史故事、寓言故事、成语故事等。

### LLM 生成优化

仅在以下情况使用 LLM：
- 用户要求定制化故事（如"讲一个关于小猫的故事"）
- 本地库和 API 都没有匹配的故事
- 用户要求续写或改编故事
- `output_mode: "summarize"` 模式

优化策略：
- 限制生成长度（max_tokens: 800）
- 使用更便宜的模型（deepseek-chat 已是性价比之选）
- 缓存生成过的故事

### 语音指令示例

```
用户: "讲个故事"
助手: [从本地库/API获取，raw模式直接朗读，零token消耗]

用户: "讲个睡前故事"
助手: [同上，零token消耗]

用户: "讲一个关于小猫的冒险故事"
助手: [LLM 生成定制故事，消耗token]

用户: "简单讲一下荆轲刺秦王的故事"
助手: [summarize模式，LLM总结后朗读]
```

## 技术方案

详见 [design.md](./design.md)

### 核心组件

1. **StoryStore** - 本地故事库管理
2. **StoryAPI** - 外部故事 API 客户端
3. **TellStoryTool** - 讲故事工具（整合三种来源）
4. **ToolResult** - 扩展工具返回结构，支持跳过 LLM

## 实现任务

详见 [tasks.md](./tasks.md)

## 影响范围

| 文件 | 变更 |
|------|------|
| `internal/tools/tool.go` | 新增 `ToolResult` 结构体 |
| `internal/tools/story.go` | 使用 `ToolResult` 返回，根据模式设置 `SkipLLM` |
| `internal/tools/story_store.go` | 新增，本地故事库 |
| `internal/tools/story_api.go` | 新增，外部 API 客户端 |
| `internal/pipeline/pipeline.go` | 检测 `SkipLLM` 标记，跳过 LLM 直接送 TTS |
| `models/stories/*.json` | 新增，故事数据文件 |
| `configs/pibuddy.yaml` | 添加故事功能配置 |

## 配置

```yaml
tools:
  story:
    enabled: true
    # 本地故事库路径
    local_path: "./models/stories"
    # 外部 API 配置
    api:
      enabled: true
      base_url: "https://www.mxnzp.com"
      app_id: "${PIBUDDY_STORY_APP_ID}"    # 可选
      app_secret: "${PIBUDDY_STORY_SECRET}" # 可选
    # LLM 生成配置
    llm_fallback: true
    max_tokens: 800
    # 输出模式配置
    output_mode: "raw"  # raw: 原文朗读（零token） | summarize: LLM总结后朗读
```

### 输出模式说明

| 模式 | 说明 | Token 消耗 | 适用场景 |
|------|------|-----------|---------|
| `raw` | 直接朗读原文，绕过 LLM | **0** | 完整体验、长故事、节省成本 |
| `summarize` | LLM 总结后朗读 | 100-500 | 快速了解、简短回复、适合短故事 |

## 预期效果

| 指标 | 优化前 | 优化后（raw模式） | 改善 |
|------|--------|-----------------|------|
| Token 消耗/故事 | 500-2000 | **0** | **-100%** |
| 响应时间 | 3-10秒 | 0.5-2秒 | **-70%** |
| 故事质量 | 不稳定 | 稳定 | 提升 |
| 离线可用 | 否 | 部分可用 | 提升 |

## 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 外部 API 不可用 | 中 | 本地库优先，LLM 兜底 |
| 本地库故事有限 | 低 | API 扩展 + LLM 定制 |
| 故事内容版权 | 低 | 使用公共领域故事 |
| Pipeline 架构变更 | 中 | 兼容现有工具返回格式 |

## 后续扩展

- [x] 故事收藏功能（SaveStoryTool）
- [x] 列出故事功能（ListStoriesTool）
- [ ] 删除本地库故事功能（DeleteStoryTool）
- [ ] 用户自定义故事导入
- [ ] 故事分类搜索
- [ ] 多语言故事支持
- [ ] 支持用户语音切换输出模式

### 删除故事功能

用户可以通过语音或命令删除本地库中的故事：

```
用户: "删除荆轲刺秦王这个故事"
助手: "已删除《荆轲刺秦王》"

用户: "删除所有用户保存的故事"
助手: "已删除 5 个用户保存的故事"
```

**DeleteStoryTool 设计：**

```yaml
# 工具参数
parameters:
  story_id:
    type: string
    description: "要删除的故事ID（可选，与 keyword 二选一）"
  keyword:
    type: string
    description: "故事关键词，用于模糊匹配（可选）"
  delete_all:
    type: boolean
    description: "是否删除所有用户保存的故事（危险操作）"
```

**安全限制：**
- 只能删除 `source=llm` 或 `source=api` 的故事（用户保存/API缓存）
- 不能删除内置故事（`source=local`）
- `delete_all=true` 时需要二次确认
- [ ] 支持用户语音切换输出模式
