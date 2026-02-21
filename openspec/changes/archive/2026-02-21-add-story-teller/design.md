# Design: 故事功能优化

## 背景

当前讲故事功能完全依赖 LLM 生成，导致 token 消耗大、响应慢、成本高。需要优化为混合方案，并在原文朗读模式下完全绕过 LLM。

## 目标 / 非目标

**目标：**
- 在 `raw` 模式下实现 **零 token 消耗**
- 减少 90%+ 的总体 token 消耗
- 提高响应速度
- 保证故事质量稳定
- 支持离线使用部分功能

**非目标：**
- 故事编辑功能
- 用户自定义故事上传（后续扩展）
- 有声故事播放（后续扩展）

## 技术决策

### 1. 故事来源优先级

```
优先级：本地库 > 外部 API > LLM 生成
```

**理由：**
- 本地库：零成本、最快响应、离线可用
- 外部 API：低成本、质量稳定、内容丰富
- LLM：高成本、定制化强、最后手段

### 2. LLM 绕过机制（核心优化）

**问题**：即使工具返回了完整故事内容，当前架构仍会经过 LLM 处理，浪费 token。

**解决方案**：扩展工具返回结构，支持标记是否跳过 LLM。

```go
// ToolResult 工具执行结果（新增）
type ToolResult struct {
    Content string `json:"content"`  // 返回内容
    SkipLLM bool   `json:"skip_llm"` // 是否跳过 LLM 直接输出
}

// ToJSON 转换为 JSON 字符串（兼容现有接口）
func (r *ToolResult) ToJSON() string {
    data, _ := json.Marshal(r)
    return string(data)
}
```

**Pipeline 处理逻辑**：

```go
// 伪代码
func (p *Pipeline) handleToolResult(result string) {
    // 尝试解析为 ToolResult
    var toolResult ToolResult
    if err := json.Unmarshal([]byte(result), &toolResult); err == nil {
        if toolResult.SkipLLM {
            // 直接送 TTS，跳过 LLM
            p.tts.Speak(toolResult.Content)
            return
        }
        // 正常流程：送 LLM 处理
        p.llm.Process(toolResult.Content)
        return
    }
    
    // 兼容旧格式：普通字符串，送 LLM 处理
    p.llm.Process(result)
}
```

**流程对比**：

```
┌─────────────────────────────────────────────────────────────────┐
│                     raw 模式（零 token）                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  用户语音 ─→ ASR ─→ LLM意图识别 ─→ 工具执行                      │
│                                         │                       │
│                                         ↓                       │
│                              ToolResult{                        │
│                                Content: "故事内容...",          │
│                                SkipLLM: true                    │
│                              }                                  │
│                                         │                       │
│                                         ↓                       │
│                              TTS 朗读 ─→ 音频输出                │
│                                                                 │
│  Token 消耗：仅意图识别阶段（~50 tokens）                         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                   summarize 模式（消耗 token）                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  用户语音 ─→ ASR ─→ LLM意图识别 ─→ 工具执行                      │
│                                         │                       │
│                                         ↓                       │
│                              ToolResult{                        │
│                                Content: "故事内容...",          │
│                                SkipLLM: false                   │
│                              }                                  │
│                                         │                       │
│                                         ↓                       │
│                              LLM 总结 ─→ TTS 朗读                │
│                                                                 │
│  Token 消耗：意图识别 + 总结（~200-500 tokens）                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 3. 本地故事库设计

**文件结构：**
```
models/stories/
├── fairy_tales.json      # 童话故事
├── fables.json           # 寓言故事
├── chinese_stories.json  # 中国传统故事
├── idiom_stories.json    # 成语故事
└── bedtime_stories.json  # 睡前故事
```

**数据格式：**
```go
type Story struct {
    ID        string   `json:"id"`
    Title     string   `json:"title"`
    Category  string   `json:"category"`
    Tags      []string `json:"tags"`
    Content   string   `json:"content"`
    WordCount int      `json:"word_count"`
    Source    string   `json:"source"` // 来源：local/api/llm
}
```

**故事匹配算法：**
```go
func (s *StoryStore) FindStory(keyword string) (*Story, error) {
    // 1. 标题精确匹配
    // 2. 标题模糊匹配
    // 3. 标签匹配
    // 4. 分类匹配
}
```

### 4. 外部 API 集成

**mxnzp 故事大全 API**

| 接口 | 说明 |
|------|------|
| `GET /api/story/types` | 获取故事分类 |
| `GET /api/story/list?type_id=1&page=1` | 获取分类下的故事列表 |
| `GET /api/story/search?keyword=xxx` | 搜索故事 |
| `GET /api/story/details?story_id=xxx` | 获取故事详情 |

响应格式：
```json
{
  "code": 1,
  "msg": "成功",
  "data": {
    "storyId": 123,
    "title": "小马过河",
    "content": "...",
    "type": "寓言故事"
  }
}
```

**缓存策略：**
- API 获取的故事缓存到本地 SQLite
- 缓存有效期：7天
- 缓存键：`api_{storyId}`

### 5. 工具接口设计

```go
// TellStoryTool 讲故事工具
type TellStoryTool struct {
    store       *StoryStore
    api         *StoryAPI
    llmFallback bool
    outputMode  string // raw 或 summarize
}

func (t *TellStoryTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    params := struct {
        Keyword string `json:"keyword"`
    }{}
    json.Unmarshal(args, &params)

    var story *Story
    
    // 1. 本地库查找
    if t.store != nil {
        story, _ = t.store.FindStory(params.Keyword)
    }
    
    // 2. API 查找
    if story == nil && t.api != nil {
        story, _ = t.api.FindStory(ctx, params.Keyword)
        if story != nil && t.store != nil {
            t.store.SaveStory(story) // 缓存
        }
    }

    if story != nil {
        return t.formatResult(story), nil
    }

    // 3. LLM 生成（兜底）
    if t.llmFallback {
        return ToolResult{
            Content: fmt.Sprintf("本地故事库和外部API都没有找到相关故事。请你自己创作一个故事。关键词：%s", params.Keyword),
            SkipLLM: false, // 需要LLM处理
        }.ToJSON(), nil
    }

    return ToolResult{
        Content: "抱歉，我没有找到相关的故事。",
        SkipLLM: true, // 直接输出
    }.ToJSON(), nil
}

// formatResult 根据输出模式格式化结果
func (t *TellStoryTool) formatResult(story *Story) string {
    content := fmt.Sprintf("《%s》\n\n%s", story.Title, story.Content)
    
    return ToolResult{
        Content: content,
        SkipLLM: t.outputMode == "raw", // raw 模式跳过 LLM
    }.ToJSON()
}
```

### 6. 输出模式配置

支持两种输出模式：

| 模式 | 说明 | SkipLLM | Token 消耗 |
|------|------|---------|-----------|
| `raw` | 原文朗读，绕过 LLM | `true` | 0 |
| `summarize` | LLM 总结后朗读 | `false` | 100-500 |

**配置项：**
```yaml
tools:
  story:
    output_mode: "raw"  # 或 "summarize"
```

**默认值：** `raw`（原文朗读，零 token）

## 实现路径

### 阶段 1：基础架构（已完成）
- ✅ 本地故事库
- ✅ 外部 API 集成
- ✅ TellStoryTool 实现

### 阶段 2：LLM 绕过机制（待实现）
1. 定义 `ToolResult` 结构体
2. 修改 `TellStoryTool.Execute` 返回 `ToolResult`
3. 修改 Pipeline 检测 `SkipLLM` 标记
4. 实现 TTS 直通逻辑

### 阶段 3：优化与测试
1. 添加单元测试
2. 性能测试（token 消耗、响应时间）
3. 文档更新

## 数据来源

**公共领域故事（无版权）：**
- 格林童话
- 安徒生童话
- 伊索寓言
- 中国古代寓言
- 成语故事

## 缓存策略

| 来源 | 缓存位置 | 有效期 |
|------|---------|--------|
| 本地库 | 文件系统 | 永久 |
| API | SQLite | 7 天 |
| LLM 生成 | SQLite | 30 天 |

## 开放问题

1. ~~是否需要绕过 LLM 节省 token？~~ → **已决定：raw 模式绕过**
2. 是否支持故事收藏？
3. 是否支持用户自定义故事导入？
4. 是否支持用户语音切换输出模式？
