## Context

PiBuddy 已有 Function Calling + Tools 架构和 Store 持久化模式（MemoStore、AlarmStore），可直接复用。RSS 功能的核心挑战是：
- Feed 解析需要处理 RSS 2.0 / Atom / RSS 1.0 等多种格式
- 需要缓存机制避免每次查询都重新抓取
- 内容可能很长，需要截断/摘要后适合语音播报

## Goals / Non-Goals

### Goals
- 用户可以通过语音添加、查看、删除 RSS 订阅源
- 用户可以通过语音获取订阅源的最新内容（标题 + 摘要）
- 支持按来源名称或关键词过滤
- 支持 RSS 2.0、Atom、RSS 1.0 格式
- 内容缓存，避免频繁请求源站

### Non-Goals
- 不做定时自动播报（后续迭代可加）
- 不做全文阅读（只返回标题和摘要）
- 不做 OPML 导入导出
- 不做已读/未读状态追踪（第一版简化）
- 不做 Feed 自动发现（需要用户提供确切 URL）

## Decisions

### 1. Feed 解析：使用 gofeed 库

**选择：`github.com/mmcdole/gofeed`**

| 方案 | 优点 | 缺点 |
|------|------|------|
| gofeed | 成熟稳定、支持 RSS/Atom/RDF、活跃维护 | 新增外部依赖 |
| 标准库 encoding/xml | 零依赖 | 需手动处理多种格式、工作量大 |

gofeed 在 Go 社区广泛使用（8k+ stars），支持所有主流 Feed 格式，带错误恢复能力。

### 2. 存储模式：复用 JSON 文件持久化

**选择：与 MemoStore / AlarmStore 相同的 JSON 文件模式**

```go
type FeedStore struct {
    mu      sync.RWMutex
    dataDir string
    feeds   []Feed
}
```

存储文件：`{dataDir}/rss_feeds.json`，包含订阅源列表。
缓存文件：`{dataDir}/rss_cache.json`，包含最近抓取的内容条目。

RSS 订阅源数量预期不多（十几个），JSON 文件完全够用。

### 3. 缓存策略

- 每个 Feed 记录 `LastFetched` 时间戳
- 调用 `get_rss_news` 时，检查缓存是否过期（默认 30 分钟）
- 过期则重新抓取并更新缓存
- 缓存每个 Feed 最近 20 条条目
- 缓存过期时间可通过配置调整

### 4. 内容截断

RSS 条目的 `Description` 可能包含 HTML 标签和很长的文本。处理策略：
- 剥离 HTML 标签，只保留纯文本
- 每条摘要截断到 200 字符（适合语音播报）
- `get_rss_news` 默认返回最新 5 条，可通过 `limit` 参数调整

### 5. 工具设计

```
add_rss_feed(url, name?)      → 添加订阅源，name 可选（自动从 Feed 标题获取）
list_rss_feeds()               → 列出所有订阅源
delete_rss_feed(id_or_name)    → 删除订阅源（支持 ID 或名称）
get_rss_news(source?, keyword?, limit?) → 获取最新内容
```

`get_rss_news` 是最常用的工具，支持：
- 不带参数：获取所有源的最新内容
- `source`：按源名称过滤
- `keyword`：按标题关键词过滤
- `limit`：返回条数（默认 5）

### 6. Pipeline 集成

与 MemoStore 完全相同的模式：
```go
feedStore, err := rss.NewFeedStore(cfg.Tools.DataDir)
p.toolRegistry.Register(tools.NewAddRSSFeedTool(feedStore))
p.toolRegistry.Register(tools.NewListRSSFeedsTool(feedStore))
p.toolRegistry.Register(tools.NewDeleteRSSFeedTool(feedStore))
p.toolRegistry.Register(tools.NewGetRSSNewsTool(feedStore))
```

仅在配置启用 RSS 时注册工具。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| 部分 RSS 源需要科学上网 | 树莓派本身需要网络，用户自行配置代理 |
| Feed 抓取可能很慢 | 设置 10 秒超时 + 缓存机制 |
| Feed 内容含 HTML/特殊字符 | 剥离 HTML 标签、转义处理 |
| 新增 gofeed 依赖增加二进制体积 | gofeed 纯 Go 实现，增量很小 |

## Open Questions

- 是否需要支持需要认证的 Feed（如 Miniflux 等自托管 RSS 服务）？（建议第一版不做）
- 后续是否需要 OPML 导入能力？（建议需求明确后再加）
