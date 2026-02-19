## 1. RSS 管理器与存储
- [x] 1.1 新增 `go get github.com/mmcdole/gofeed` 依赖
- [x] 1.2 新建 `internal/rss/feed.go` — 定义 `Feed`（ID, Name, URL, AddedAt, LastFetched）和 `FeedItem`（Title, Summary, Link, Published, FeedName）结构体
- [x] 1.3 新建 `internal/rss/store.go` — `FeedStore` 实现：订阅源 CRUD（Add/List/Delete）+ JSON 文件持久化（`rss_feeds.json`）
- [x] 1.4 新建 `internal/rss/fetcher.go` — `Fetcher` 实现：使用 gofeed 解析 Feed、HTML 标签剥离、内容截断、缓存管理（`rss_cache.json`，默认 30 分钟过期）
- [x] 1.5 新建 `internal/rss/store_test.go` — FeedStore 单元测试（Add/List/Delete、持久化、并发安全）
- [x] 1.6 新建 `internal/rss/fetcher_test.go` — Fetcher 单元测试（用 httptest mock RSS 响应、缓存命中/过期、HTML 剥离、内容截断）

## 2. LLM 工具
- [x] 2.1 新建 `internal/tools/rss.go` — 实现 4 个工具：
  - `AddRSSFeedTool` — 参数 `url`（必填）、`name`（可选），添加时自动抓取一次验证 URL 有效性
  - `ListRSSFeedsTool` — 无必填参数，返回所有订阅源列表
  - `DeleteRSSFeedTool` — 参数 `id`（订阅源 ID 或名称）
  - `GetRSSNewsTool` — 参数 `source`（可选）、`keyword`（可选）、`limit`（可选，默认 5）
- [x] 2.2 新建 `internal/tools/rss_test.go` — 工具执行逻辑测试（用 httptest mock Feed 数据）

## 3. 配置与集成
- [x] 3.1 修改 `internal/config/config.go` — 新增 `RSSConfig`（`Enabled bool`、`CacheTTL int`）到 `ToolsConfig`
- [x] 3.2 修改 `configs/pibuddy.yaml` — 新增 `rss` 配置段
- [x] 3.3 修改 `internal/pipeline/pipeline.go` — 在 `initTools()` 中条件注册 RSS 工具（仅 `rss.enabled: true` 时）

## 4. 测试与验证
- [x] 4.1 确保 `go test ./...` 全部通过
- [x] 4.2 确保 `go build ./...` 编译通过
