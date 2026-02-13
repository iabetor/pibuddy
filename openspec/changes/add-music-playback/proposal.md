# Change: 新增音乐播放能力（QQ 音乐语音点歌）

## Why

用户（小朋友）希望通过语音对 PiBuddy 说"播放小星星"等指令来点歌播放。当前 PiBuddy 只有对话能力，所有 ASR 结果都直接送 LLM 再 TTS 播放文本回复，没有技能分发机制，也没有音频流播放能力。

需要新增：
1. 意图识别 — 区分"闲聊"和"播放音乐"
2. 音乐搜索 — 通过第三方 API 搜索歌曲并获取播放 URL
3. 音频流播放 — 直接播放 MP3/音频流（区别于现有的 TTS float32 样本播放）

## What Changes

- **新增 `internal/skill/` 包** — 技能/插件分发层，位于 LLM 和 TTS 之间
- **新增 `internal/music/` 包** — 音乐搜索和 URL 获取客户端（封装第三方 API）
- **新增 `internal/audio/stream.go`** — 音频流播放能力（HTTP URL → 解码 → PCM → 播放）
- **修改 `internal/pipeline/pipeline.go`** — `processQuery` 中插入意图分发逻辑
- **修改 `internal/pipeline/state.go`** — 新增 `StatePlaying` 状态（区别于 TTS 的 `StateSpeaking`），或复用 `StateSpeaking`
- **修改 `internal/config/config.go`** — 新增 `MusicConfig` 配置项

### 技术路线

音乐 API 采用社区开源方案（如 Meting 协议），部署为本地 HTTP 服务或直接 Go 实现 HTTP 客户端调用 QQ 音乐 Web 端接口。

**已知风险**：
- 非官方接口可能随时失效，需要做好降级处理（搜索失败时语音回复"暂时无法播放"）
- VIP 歌曲需要手动提供登录 Cookie，Cookie 会过期
- 免费歌曲可直接获取播放 URL

## Impact

- 新增 specs: `music-playback`
- 受影响代码:
  - `internal/pipeline/pipeline.go` — processQuery 流程改造
  - `internal/pipeline/state.go` — 可能新增状态
  - `internal/config/config.go` — 新增配置结构
  - `internal/audio/` — 新增流式音频播放
- 新增代码:
  - `internal/skill/` — 技能分发
  - `internal/music/` — 音乐 API 客户端
