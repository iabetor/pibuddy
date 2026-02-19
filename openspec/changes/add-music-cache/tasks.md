## 1. 缓存核心模块
- [x] 1.1 新建 `internal/audio/cache.go` — `MusicCache` 结构体（索引加载/保存、Lookup、Store、Search、Evict）
- [ ] 1.2 新建 `internal/audio/cache_test.go` — 测试缓存索引 CRUD、模糊搜索、LRU 淘汰逻辑
- [x] 1.3 `MusicCache.Search(keyword)` 实现本地模糊搜索（name/artist 匹配），返回匹配的缓存条目列表

## 2. 配置扩展
- [x] 2.1 修改 `internal/config/config.go` — `MusicConfig` 增加 `CacheDir string` 和 `CacheMaxSize int64`（MB）
- [x] 2.2 修改 `internal/config/config.go` — `setDefaults` 中设置默认值（`CacheDir` = `{DataDir}/music_cache`，`CacheMaxSize` = 500）
- [x] 2.3 更新 `configs/pibuddy.yaml` — 新增 `cache_dir` 和 `cache_max_size` 配置

## 3. StreamPlayer 缓存集成
- [x] 3.1 新增 `PlayOptions` 结构体和 `StreamPlayer.Play()` 签名变更（增加 `opts *PlayOptions`）
- [x] 3.2 实现缓存命中时从本地文件播放的 `playFromFile` 方法
- [x] 3.3 实现网络下载时 tee 写入本地缓存文件（.tmp → rename 原子写入）
- [x] 3.4 播放完成后调用 `MusicCache.Store()` 更新索引

## 4. PlayMusicTool 本地优先
- [x] 4.1 修改 `internal/tools/music.go` — `MusicConfig` 增加 `Cache *audio.MusicCache` 字段
- [x] 4.2 修改 `PlayMusicTool.Execute()` — 先调用 `cache.Search(keyword)`，命中则直接构建 PlaylistItem 跳过网络
- [x] 4.3 未命中时走原有流程，PlaylistItem 携带 CacheKey（`{provider}_{songID}`）
- [x] 4.4 `MusicResult` 增加 `CacheKey string` 字段

## 5. Playlist 和 Pipeline 适配
- [x] 5.1 修改 `internal/music/playlist.go` — `PlaylistItem` 增加 `CacheKey string` 字段
- [x] 5.2 修改 `internal/pipeline/pipeline.go` — `initTools` 中创建 `MusicCache` 并传入 `MusicConfig`
- [x] 5.3 修改 `internal/pipeline/pipeline.go` — `playMusic()` 传递 `PlayOptions`（含 CacheKey 和 Cache）
- [x] 5.4 自动切换下一首时同样传递 `PlayOptions`

## 6. 测试与验证
- [x] 6.1 确保 `go build ./...` 编译通过
- [x] 6.2 确保 `go test ./...` 全部通过
- [ ] 6.3 手动测试：首次播放歌曲 → 验证缓存文件和索引写入
- [ ] 6.4 手动测试：再次播放同一首歌 → 验证走本地缓存（日志中无 HTTP 下载）
- [ ] 6.5 手动测试：用关键词搜索已缓存歌曲 → 验证离线播放（关闭音乐 API 服务）
