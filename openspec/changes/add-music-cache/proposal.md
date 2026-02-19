# Change: 新增音乐本地缓存（避免重复下载，支持离线播放）

## Why

当前每次播放歌曲都是纯网络流式：HTTP 下载 → 内存缓冲 → MP3 解码 → 播放，播放结束后数据全部丢弃。同一首歌再次播放时需要重新从网络下载。在树莓派上存在以下问题：

1. **重复下载浪费带宽**：相同歌曲每次都要重新下载 3-5MB
2. **启动延迟**：每次都要等待网络 RTT + 首批 32KB 缓冲
3. **依赖外部服务**：QQ 音乐 / 网易云 API 不可用时完全无法播放
4. **无法离线播放**：已经听过的歌曲在断网时也无法播放

## What Changes

- **新增 `internal/audio/cache.go`** — `MusicCache` 结构体，管理本地 MP3 文件缓存（存储、查找、模糊搜索、LRU 淘汰）
- **修改 `internal/audio/stream.go`** — `StreamPlayer.Play()` 增加缓存参数，支持缓存命中直接本地播放、缓存未命中时边播边存
- **修改 `internal/tools/music.go`** — `PlayMusicTool` 先查本地缓存，命中则跳过网络搜索；未命中走原有流程
- **修改 `internal/config/config.go`** — `MusicConfig` 增加 `CacheDir` 和 `CacheMaxSize` 配置
- **修改 `configs/pibuddy.yaml`** — 新增缓存相关配置项
- **修改 `internal/pipeline/pipeline.go`** — `playMusic()` 传递缓存参数
- **修改 `internal/music/playlist.go`** — `PlaylistItem` 增加 `CacheKey` 字段

### 关键设计点

1. **缓存索引包含歌曲元信息**：不仅缓存 MP3 文件，还记录歌名、歌手、专辑等，支持本地模糊搜索
2. **本地优先**：先查本地缓存索引匹配，命中则完全跳过外部 API，实现离线播放
3. **高品质覆盖**：当用户开通 VIP 后重新搜索播放同一首歌，网络下载的新文件（可能是更高品质）会覆盖已缓存的旧文件
4. **原子写入**：先写 `.tmp` 文件，下载完整后 rename，避免残缺文件
5. **LRU 淘汰**：缓存总大小超过上限时按最后播放时间淘汰最老的文件

## Impact

- 修改 specs: `music-playback`（新增缓存相关需求）
- 受影响代码:
  - `internal/audio/stream.go` — Play 方法签名变更，增加缓存读写逻辑
  - `internal/audio/cache.go` — **新文件**
  - `internal/tools/music.go` — PlayMusicTool 增加本地缓存查找
  - `internal/config/config.go` — MusicConfig 增加字段
  - `internal/pipeline/pipeline.go` — playMusic 传递缓存参数
  - `internal/music/playlist.go` — PlaylistItem 增加 CacheKey
  - `configs/pibuddy.yaml` — 新增配置
