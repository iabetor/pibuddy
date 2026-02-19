## ADDED Requirements

### Requirement: 音乐本地缓存

系统 SHALL 将播放过的音乐文件缓存到本地磁盘，再次播放同一首歌时直接从本地文件读取，避免重复网络下载。

#### Scenario: 首次播放写入缓存
- **WHEN** 用户播放一首未缓存的歌曲
- **THEN** 系统从网络流式下载并播放（行为与原来一致）
- **THEN** 下载完成后将 MP3 文件保存到缓存目录（原子写入：先写 .tmp 再 rename）
- **THEN** 更新缓存索引，记录歌曲元信息（ID、歌名、歌手、专辑、文件大小、缓存时间）

#### Scenario: 再次播放命中缓存
- **WHEN** 用户播放一首已缓存的歌曲
- **THEN** 系统直接从本地文件读取并播放，不发起网络请求
- **THEN** 更新缓存索引中的 `last_played` 时间

#### Scenario: 高品质文件覆盖
- **WHEN** 用户通过网络搜索播放一首已缓存的歌曲（如开通 VIP 后重新搜索）
- **THEN** 系统从网络下载新文件并覆盖已有缓存文件
- **THEN** 缓存索引中对应条目更新文件大小和缓存时间

#### Scenario: 播放中断不写入缓存
- **WHEN** 歌曲播放被唤醒词打断或因网络错误未下载完整
- **THEN** 不写入缓存文件（.tmp 文件被清理）
- **THEN** 已有缓存不受影响

### Requirement: 本地缓存优先搜索

系统 SHALL 在播放音乐时先查询本地缓存索引，命中则完全跳过外部音乐 API 调用，实现离线播放已缓存歌曲。

#### Scenario: 本地搜索命中
- **WHEN** 用户说"播放晴天"
- **WHEN** 本地缓存中存在歌名或歌手匹配"晴天"的条目
- **THEN** 系统直接使用本地缓存文件构建播放列表，不调用外部 API
- **THEN** 开始播放

#### Scenario: 本地搜索未命中
- **WHEN** 用户说"播放一首新歌"
- **WHEN** 本地缓存中无匹配条目
- **THEN** 系统走原有流程：调用外部 API 搜索 → 获取 URL → 流式播放 + 写缓存

#### Scenario: 外部 API 不可用但有缓存
- **WHEN** 音乐 API 服务不可用（网络断开或服务关闭）
- **WHEN** 用户请求播放的歌曲在本地缓存中存在
- **THEN** 系统从本地缓存播放，不报错

### Requirement: 缓存空间管理

系统 SHALL 限制缓存总大小，超限时自动淘汰最久未播放的缓存文件。

#### Scenario: 缓存超限自动淘汰
- **WHEN** 写入新缓存文件后总缓存大小超过 `cache_max_size` 配置值
- **THEN** 系统按 `last_played` 升序删除最久未播放的缓存文件
- **THEN** 直到总缓存大小低于上限

#### Scenario: 禁用缓存
- **WHEN** `cache_max_size` 配置为 0
- **THEN** 系统不缓存任何音乐文件，行为与未加缓存前完全一致

## MODIFIED Requirements

### Requirement: 音乐功能配置

音乐功能 SHALL 通过配置文件启用和配置。

#### Scenario: 配置启用音乐
- **WHEN** 配置文件中 `music.enabled` 为 `true` 且 `music.api_url` 已设置
- **THEN** 系统启动时初始化音乐 Provider

#### Scenario: 未配置时降级
- **WHEN** 配置文件中 `music.enabled` 为 `false` 或未设置
- **THEN** 音乐相关语音请求走普通对话流程（LLM 文字回复）

#### Scenario: 配置缓存参数
- **WHEN** 配置文件中 `music.cache_dir` 和 `music.cache_max_size` 已设置
- **THEN** 系统启动时初始化 MusicCache，使用指定目录和大小限制
- **WHEN** 缓存相关配置未设置
- **THEN** 使用默认值：`cache_dir` = `{data_dir}/music_cache`，`cache_max_size` = 500（MB）
