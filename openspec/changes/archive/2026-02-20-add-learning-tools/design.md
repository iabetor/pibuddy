## Context

PiBuddy 是一个语音助手，运行在树莓派上。本项目添加学习工具集，包括英语学习、汉字拼音、古诗词功能。

**约束条件**：
- API 调用应尽量少（网络延迟影响语音响应）
- 免费额度有限，需要缓存策略
- 本地存储应轻量（JSON 文件）

## Goals / Non-Goals

**Goals**：
- 提供实用的语音学习工具
- 最小化 API 依赖和网络调用
- 支持离线使用（拼音功能）
- 趣味性（诗词游戏）

**Non-Goals**：
- 不实现完整的在线教育平台
- 不支持多用户（暂无用户系统）
- 不实现复杂的发音评测算法

## Decisions

### 1. API 选择

| 功能 | 选择 | 原因 |
|------|------|------|
| 单词查询 | 有道词典 | 免费、无需 key、稳定 |
| 每日一句 | 金山词霸 | 免费、格式规范 |
| 古诗词 | 诗词六六六 | 10万次/月，数据全 |
| 拼音 | go-pinyin | 本地库，无网络延迟 |

**替代方案**：
- 天聚数行 API（需申请 key，额度少）
- 聚合数据 API（需申请 key）

### 2. 存储设计

使用 JSON 文件存储，简单可靠：

```json
// vocabulary.json - 生词本
{
  "words": [
    {
      "word": "hello",
      "phonetic": "/həˈləʊ/",
      "meaning": "你好",
      "added_at": "2026-02-20T10:00:00Z"
    }
  ]
}

// poetry_cache.json - 诗词缓存
{
  "daily": {
    "2026-02-20": {
      "title": "静夜思",
      "author": "李白",
      "content": "床前明月光..."
    }
  },
  "search_results": {
    "月亮": [...]  // 搜索结果缓存
  }
}

// word_bank.json - 测验词库
{
  "words": [
    {"word": "abandon", "meaning": "放弃", "level": "cet4"},
    {"word": "absorb", "meaning": "吸收", "level": "cet4"}
  ]
}
```

### 3. 工具接口设计

```go
// 单词查询
type EnglishWordArgs struct {
    Word string `json:"word"`
}

// 每日一句 - 无参数

// 生词本操作
type VocabularyArgs struct {
    Action string `json:"action"` // add, list, remove
    Word   string `json:"word"`   // add/remove 时必需
}

// 单词测验
type QuizArgs struct {
    Action string `json:"action"` // start, answer
    Answer string `json:"answer"` // answer 时必需
}

// 拼音查询
type PinyinArgs struct {
    Text   string `json:"text"`
    Tone   bool   `json:"tone"`   // 是否带声调，默认 true
}

// 诗词搜索
type PoetrySearchArgs struct {
    Query    string `json:"query"`
    Type     string `json:"type"`     // keyword, author, sentence
    Page     int    `json:"page"`
    PageSize int    `json:"page_size"`
}

// 飞花令/接龙
type PoetryGameArgs struct {
    Game    string `json:"game"`    // feihualing, jielong
    Keyword string `json:"keyword"` // 飞花令关键字
    Input   string `json:"input"`   // 用户输入的诗句
}
```

### 4. 诗词游戏逻辑

**飞花令**：
1. 用户发起游戏，指定关键字（如"花"）
2. AI 先背一句含"花"的诗
3. 用户接一句含"花"的诗
4. AI 验证用户输入是否含关键字，再回复下一句
5. 重复直到一方接不上

**诗词接龙**：
1. 用户发起游戏
2. AI 背一句诗
3. 用户背一句诗，首字需与上一句尾字相同（同音即可）
4. AI 验证并回复
5. 重复直到一方接不上

**AI 回复策略**：
- 先从缓存中查找含关键字的诗句
- 缓存没有则调用 API 搜索
- 记录已使用的诗句避免重复

### 5. 缓存策略

| 数据类型 | 缓存时间 | 原因 |
|---------|---------|------|
| 每日一句 | 24 小时 | 每天更新一次 |
| 每日一诗 | 24 小时 | 每天更新一次 |
| 单词释义 | 永久 | 不变数据 |
| 诗词搜索结果 | 7 天 | 降低 API 调用 |

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| API 调用失败 | 返回友好提示，建议稍后重试 |
| 网络延迟影响响应 | 拼音功能优先（本地） |
| 诗词 API 配额用尽 | 缓存常用诗词，降级到本地库 |
| 生词本文件损坏 | 启动时校验，损坏则重建 |
| 测验词库不够 | 支持用户添加生词到测验 |

## Migration Plan

无需迁移，纯新增功能。

## Open Questions

- [ ] 是否需要支持成语接龙？
- [ ] 是否需要学习进度统计？
- [ ] 测验是否支持不同难度级别？
