# Change: 新增 RSS 订阅与语音播报功能

## Why

PiBuddy 作为智能语音助手，用户希望能通过语音订阅感兴趣的信息源（科技资讯、新闻博客等），并在需要时通过语音获取最新内容摘要。当前 PiBuddy 只有实时新闻（腾讯热点新闻）和天气等工具，缺少用户自定义信息订阅能力。

RSS/Atom 是互联网上最通用的信息订阅协议，支持绝大多数新闻站、博客、播客目录等。新增 RSS 功能可以让用户：
1. 通过语音添加/管理订阅源
2. 随时询问"有什么新消息"获取未读内容摘要
3. 按来源或关键词过滤信息

## What Changes

- **新增 `internal/rss/` 包** — RSS/Atom Feed 解析、订阅源管理、内容缓存
- **新增 `internal/tools/rss.go`** — 4 个 LLM 工具：`add_rss_feed`、`list_rss_feeds`、`delete_rss_feed`、`get_rss_news`
- **修改 `internal/config/config.go`** — 新增 `RSSConfig` 配置项
- **修改 `internal/pipeline/pipeline.go`** — 注册 RSS 工具
- **修改 `configs/pibuddy.yaml`** — 新增 RSS 配置段
- **新增依赖** `github.com/mmcdole/gofeed` — 成熟的 Go RSS/Atom 解析库

## Impact

- 新增 specs: `rss-subscription`
- 受影响代码:
  - `internal/config/config.go` — 新增配置结构
  - `internal/pipeline/pipeline.go` — 注册工具
  - `configs/pibuddy.yaml` — 新增配置
- 新增代码:
  - `internal/rss/` — RSS 管理器和存储
  - `internal/tools/rss.go` — RSS 工具集
