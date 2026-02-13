# Change: 新增音乐播放能力（网易云音乐语音点歌）

## Why

用户（小朋友）希望通过语音对 PiBuddy 说"播放小星星"等指令来点歌播放。当前 PiBuddy 只有对话能力，所有 ASR 结果都直接送 LLM 再 TTS 播放文本回复，没有音频流播放能力。

需要新增：
1. 音乐搜索 — 通过网易云音乐 API 搜索歌曲并获取播放 URL
2. 音频流播放 — 直接播放 MP3 音频流（区别于现有的 TTS float32 样本播放）

## What Changes

- **新增 `internal/music/` 包** — 音乐搜索和 URL 获取客户端（封装 NeteaseCloudMusicApi）
- **新增 `internal/audio/stream.go`** — 音频流播放能力（HTTP URL → 解码 → PCM → 播放）
- **修改 `internal/tools/registry.go`** — 新增 `play_music` 工具
- **修改 `internal/config/config.go`** — 新增 `MusicConfig` 配置项
- **修改 `configs/pibuddy.yaml`** — 新增音乐配置段

### 技术路线

音乐 API 采用 [NeteaseCloudMusicApi](https://gitlab.com/Binaryify/NeteaseCloudMusicApi)（社区备份版），部署为本地 HTTP 服务。

**已验证**：在本机测试通过，API 可正常搜索歌曲并获取播放 URL。

**前置条件**：
- 用户需自行部署 NeteaseCloudMusicApi 服务（Node.js）
- PiBuddy 通过 HTTP 调用该服务

**已知风险**：
- 非官方接口可能随时失效，需要做好降级处理（搜索失败时语音回复"暂时无法播放"）
- 部分歌曲需要 VIP 才能获取播放 URL
- 项目官方已因版权问题停止维护，使用社区备份版

## Impact

- 新增 specs: `music-playback`
- 受影响代码:
  - `internal/tools/registry.go` — 新增 play_music 工具
  - `internal/config/config.go` — 新增配置结构
  - `internal/audio/` — 新增流式音频播放
- 新增代码:
  - `internal/music/` — 音乐 API 客户端
