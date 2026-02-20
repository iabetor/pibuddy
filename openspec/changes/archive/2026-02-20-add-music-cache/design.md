## Context

PiBuddy 的音乐播放当前是纯流式架构：每次播放都从网络下载 MP3，在内存中解码播放，结束后数据丢弃。目标是加一层透明的本地文件缓存，让播放过的歌曲下次可以直接从本地文件播放，同时支持高品质文件覆盖旧缓存。

核心约束：
- 树莓派 SD 卡空间有限（通常 32-64GB），需要控制缓存上限
- 不能影响现有流式播放的低延迟体验
- QQ 音乐 / 网易云的播放 URL 有时效性（通常几小时过期），缓存的是文件而非 URL

## Goals / Non-Goals

### Goals
- 播放过的歌曲缓存到本地，再次播放时零网络延迟
- 缓存索引包含歌曲元信息，支持本地模糊搜索，实现离线播放已缓存歌曲
- 缓存总大小可配置，超限时自动 LRU 淘汰
- 高品质文件可覆盖已缓存的低品质文件（比如开了会员后重新播放）

### Non-Goals
- 不做主动下载（只缓存播放过的歌曲）
- 不做跨设备缓存同步
- 不做缓存的手动管理 UI（只提供配置和自动淘汰）

## Decisions

### 1. 缓存文件命名：`{provider}_{songID}.mp3`

```
~/.pibuddy/music_cache/
├── qq_12345678.mp3
├── netease_1901234.mp3
└── cache_index.json
```

- 用 provider + songID 保证唯一性
- 同一首歌重新下载时直接覆盖同名文件（实现高品质替换）
- `.mp3` 后缀方便调试和手动查看

### 2. 缓存索引：JSON 文件 + 内存索引

```json
{
  "qq_12345678": {
    "id": 12345678,
    "name": "晴天",
    "artist": "周杰伦",
    "album": "叶惠美",
    "provider": "qq",
    "size": 4552613,
    "cached_at": "2026-02-19T17:20:38+08:00",
    "last_played": "2026-02-19T17:20:38+08:00"
  }
}
```

- 启动时加载到内存，写入时持久化
- 支持按 name/artist 模糊匹配（简单的 `strings.Contains`）
- `last_played` 用于 LRU 淘汰排序

### 3. 播放流程：两级查找

```
用户说"播放晴天"
       │
       ▼
PlayMusicTool.Execute(keyword="晴天")
       │
       ├── 1. cache.Search("晴天") → 命中
       │      └── 直接构建 PlaylistItem{CacheKey: "qq_12345678"}
       │      └── 完全跳过 provider.Search + GetSongURL
       │
       └── 2. cache.Search("晴天") → 未命中
              └── 走原有流程: provider.Search → GetSongURL → 构建 PlaylistItem
              └── PlaylistItem 携带 CacheKey 用于播放时写缓存

Pipeline.playMusic(url, cacheKey)
       │
       ├── cacheKey 非空 + 本地文件存在 → playFromFile(localPath)
       │      └── 更新 last_played
       │
       └── cacheKey 非空 + 本地文件不存在 → playFromNetwork(url)
              └── 边下载边播放（原有逻辑）
              └── 下载完成后写入缓存文件（.tmp → rename）
              └── 更新缓存索引
```

### 4. 高品质覆盖策略

当用户开通 VIP 后重新搜索播放同一首歌：
- `PlayMusicTool.Execute` 中 `cache.Search("晴天")` 命中已缓存歌曲
- 但用户实际上是通过语音"播放晴天"触发的，此时 **仍然优先走本地缓存**
- 如果想更新缓存，需要网络搜索到新 URL 后重新下载

**实际流程**：当 `cache.Search` 命中但同时也拿到了网络 URL（比如 playlist 中已有 URL）时，比较本地文件大小与网络 Content-Length：
- 网络文件更大 → 重新下载并覆盖（可能是更高品质）
- 网络文件相同或更小 → 使用本地缓存

**更简单的方案**（推荐）：缓存命中时直接使用本地文件，不做大小比较。用户如果想强制更新缓存，可以删除缓存目录或在配置中设置 `cache_refresh: true`。实际上，当 `PlayMusicTool` 通过网络搜索到歌曲时（缓存未命中的路径），会自动下载最新品质的文件并覆盖旧缓存。所以用户只需要等缓存过期后自然更新即可。

**最终决定**：网络搜索路径始终写缓存（覆盖已有文件）。这意味着：
- 缓存命中 → 本地播放（快速）
- 缓存未命中 → 网络播放 + 写缓存（首次）
- 同一首歌下次通过网络搜索触达 → 网络播放 + 覆盖缓存文件（品质可能提升）

### 5. LRU 淘汰

- 触发时机：每次写入新缓存文件后检查总大小
- 淘汰策略：按 `last_played` 升序排列，删除最久未播放的文件，直到总大小低于上限
- 默认上限：500MB（约 100-150 首歌）

### 6. StreamPlayer.Play 签名变更

```go
// 旧
func (sp *StreamPlayer) Play(ctx context.Context, url string) error

// 新
func (sp *StreamPlayer) Play(ctx context.Context, url string, opts *PlayOptions) error

type PlayOptions struct {
    CacheKey string      // 缓存标识，如 "qq_12345678"
    Cache    *MusicCache // 缓存管理器（nil 则不缓存）
}
```

使用 options struct 避免破坏已有调用方，`opts` 为 nil 时行为与原来完全一致。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| SD 卡空间不足 | 可配置 `cache_max_size`，默认 500MB，设为 0 禁用缓存 |
| 缓存索引损坏 | 启动时校验索引，文件不存在的条目自动清理 |
| 并发写入冲突 | 原子写入（.tmp → rename）+ 索引加锁 |
| 本地搜索精度不够 | 简单 Contains 匹配，多个命中时按 last_played 倒序取最近播放的 |

## Open Questions

- 是否需要提供语音指令清理缓存？（如"清理音乐缓存"）
- 缓存条目是否需要设置最大过期时间？（如 30 天未播放自动清理）
