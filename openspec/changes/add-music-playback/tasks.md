## 1. 音乐 API 客户端
- [x] 1.1 新建 `internal/music/provider.go` — 定义 `Provider` 接口（`Search`, `GetSongURL`）和 `Song` 结构体
- [x] 1.2 新建 `internal/music/netease.go` — 实现网易云音乐 API 客户端（搜索、获取播放 URL）
- [x] 1.3 新建 `internal/music/netease_test.go` — 用 httptest mock 测试搜索和 URL 获取
- [x] 1.4 新增 `MusicConfig` 到 `internal/config/config.go`（api_url, enabled）

## 2. 音频流播放
- [x] 2.1 新建 `internal/audio/stream.go` — `StreamPlayer` 结构体，支持从 HTTP URL 流式下载 + MP3 解码 + PCM 播放
- [x] 2.2 新建 `internal/audio/stream_test.go` — 用 httptest 提供静态 MP3 数据测试解码流程
- [x] 2.3 确认 `go-mp3` 解码后的 PCM 格式与现有 `Player` 采样率兼容，必要时加重采样

## 3. 工具注册
- [x] 3.1 新建 `internal/tools/music.go` — 实现 `play_music` 工具，调用 music.Provider 搜索并返回歌曲 URL
- [x] 3.2 修改 `internal/tools/registry.go` — 注册 `play_music` 工具
- [x] 3.3 新建 `internal/tools/music_test.go` — 测试工具执行逻辑

## 4. Pipeline 集成
- [x] 4.1 修改 `internal/pipeline/pipeline.go` — 检测 `play_music` 工具调用结果，触发流式播放
- [x] 4.2 Pipeline 中集成 `StreamPlayer`，音乐播放走流式路径，TTS 走现有路径
- [x] 4.3 音乐播放期间复用现有唤醒词打断机制（StateSpeaking + cancelSpeak）
- [x] 4.4 搜索失败或播放失败时 TTS 语音提示"抱歉，暂时无法播放"

## 5. 配置与部署
- [x] 5.1 更新 `configs/pibuddy.yaml` — 新增 `music` 配置段
- [x] 5.2 新建 `scripts/setup-music.sh` — NeteaseCloudMusicApi 部署脚本（可选安装）
- [x] 5.3 补充音乐相关单元测试，确保 `make test` 全部通过

