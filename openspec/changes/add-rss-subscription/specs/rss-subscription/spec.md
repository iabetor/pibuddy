## ADDED Requirements

### Requirement: RSS 订阅源管理
系统 SHALL 支持用户通过语音添加、查看和删除 RSS/Atom 订阅源。订阅源信息 SHALL 持久化存储到本地 JSON 文件（`{dataDir}/rss_feeds.json`）。

#### Scenario: 添加订阅源成功
- **WHEN** 用户通过语音说"帮我订阅 36氪的RSS"，LLM 调用 `add_rss_feed` 工具传入 URL
- **THEN** 系统抓取 Feed 验证 URL 有效性，自动获取 Feed 标题作为名称，保存到订阅列表，返回"已成功订阅 36氪"

#### Scenario: 添加订阅源时 URL 无效
- **WHEN** LLM 调用 `add_rss_feed` 传入无效 URL
- **THEN** 系统返回错误信息"无法解析该 RSS 地址，请检查 URL 是否正确"

#### Scenario: 添加重复订阅源
- **WHEN** LLM 调用 `add_rss_feed` 传入已存在的 URL
- **THEN** 系统返回提示"该订阅源已存在"

#### Scenario: 列出所有订阅源
- **WHEN** 用户问"我订阅了哪些东西"，LLM 调用 `list_rss_feeds`
- **THEN** 系统返回所有订阅源的名称和 URL 列表

#### Scenario: 删除订阅源
- **WHEN** 用户说"取消订阅 36氪"，LLM 调用 `delete_rss_feed` 传入名称或 ID
- **THEN** 系统从订阅列表中删除该源，返回"已取消订阅 36氪"

### Requirement: RSS 内容获取
系统 SHALL 支持获取订阅源的最新内容条目，返回标题和纯文本摘要，适合语音播报。系统 SHALL 使用缓存机制避免频繁请求源站。

#### Scenario: 获取所有源最新内容
- **WHEN** 用户问"有什么新消息"，LLM 调用 `get_rss_news` 不带过滤参数
- **THEN** 系统返回所有订阅源最新的 5 条内容（标题 + 摘要），缓存未过期时直接返回缓存数据

#### Scenario: 按来源过滤
- **WHEN** LLM 调用 `get_rss_news` 传入 `source="36氪"`
- **THEN** 系统仅返回 36氪 订阅源的最新内容

#### Scenario: 按关键词过滤
- **WHEN** LLM 调用 `get_rss_news` 传入 `keyword="AI"`
- **THEN** 系统返回标题中包含"AI"关键词的内容条目

#### Scenario: 缓存命中
- **WHEN** 距离上次抓取不超过缓存 TTL（默认 30 分钟）
- **THEN** 系统直接返回缓存内容，不发起网络请求

#### Scenario: 缓存过期
- **WHEN** 距离上次抓取超过缓存 TTL
- **THEN** 系统重新抓取 Feed 内容，更新缓存后返回

#### Scenario: 无订阅源时获取内容
- **WHEN** 用户没有任何订阅源时调用 `get_rss_news`
- **THEN** 系统返回"你还没有添加任何 RSS 订阅，可以告诉我想订阅的网站地址"

### Requirement: RSS 内容处理
系统 SHALL 对 Feed 内容进行清理和截断处理，确保适合语音播报。

#### Scenario: HTML 标签剥离
- **WHEN** Feed 条目的描述包含 HTML 标签
- **THEN** 系统剥离所有 HTML 标签，只保留纯文本

#### Scenario: 内容截断
- **WHEN** Feed 条目摘要超过 200 字符
- **THEN** 系统截断到 200 字符并添加省略号

### Requirement: RSS 配置
系统 SHALL 支持通过配置文件控制 RSS 功能的启用和缓存参数。

#### Scenario: 功能启用
- **WHEN** 配置 `tools.rss.enabled: true`
- **THEN** Pipeline 注册 RSS 相关的 4 个工具

#### Scenario: 功能禁用
- **WHEN** 配置 `tools.rss.enabled: false` 或未配置
- **THEN** Pipeline 不注册 RSS 工具，LLM 不可调用 RSS 功能

#### Scenario: 自定义缓存时间
- **WHEN** 配置 `tools.rss.cache_ttl: 60`
- **THEN** 缓存有效期为 60 分钟
