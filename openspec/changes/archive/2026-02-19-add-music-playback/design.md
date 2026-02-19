## Context

PiBuddy 当前是纯对话型语音助手，所有语音输入经 ASR → LLM → TTS 播放文本回复。已有 Function Calling + Tools 架构，可复用来实现音乐播放。

核心挑战：
- 当前 `audio.Player.Play()` 只接受 float32 样本，音乐播放需要从 HTTP URL 流式解码
- 音乐播放时长远大于 TTS（几分钟 vs 几秒），需要支持长时间播放和打断

## Goals / Non-Goals

### Goals
- 用户可以通过语音点歌（"播放小星星"、"我想听周杰伦的歌"）
- 音乐播放期间支持唤醒词打断（复用现有打断机制）
- 搜索失败或 API 不可用时语音回复降级提示
- 可配置音乐 API 地址（方便切换不同后端）

### Non-Goals
- 不做歌单管理、收藏、推荐等复杂功能
- 不做多音源聚合（只接网易云 API）
- 不做歌词显示
- 不做网易云账号登录流程

## Decisions

### 1. 意图识别：复用现有 Function Calling + Tools 架构

**选择：新增 `play_music` 工具**

项目已有 Function Calling + Tools 架构，直接新增一个工具：
```go
{
    "type": "function",
    "function": {
        "name": "play_music",
        "description": "播放音乐。当用户想听歌、播放音乐时调用。",
        "parameters": {
            "type": "object",
            "properties": {
                "keyword": {"type": "string", "description": "歌曲名或歌手名"}
            },
            "required": ["keyword"]
        }
    }
}
```

### 2. 音乐 API：NeteaseCloudMusicApi

**选择：用户自行部署 NeteaseCloudMusicApi 服务，PiBuddy HTTP 调用**

| 方案 | 优点 | 缺点 |
|------|------|------|
| 用户部署 NeteaseCloudMusicApi | 曲库丰富、API 成熟（200+ 接口） | 需要用户额外部署 Node.js 服务 |
| PiBuddy 内嵌实现 | 开箱即用 | 维护成本高、接口变更需频繁更新 |

部署步骤：
```bash
# 用户执行
git clone https://gitcode.com/gh_mirrors/ne/NeteaseCloudMusicApiBackup.git
cd NeteaseCloudMusicApi
npm install && node app.js  # 监听 http://localhost:3000
```

API 封装：
```go
type Provider interface {
    Search(ctx context.Context, keyword string, limit int) ([]Song, error)
    GetSongURL(ctx context.Context, songID int64) (string, error)
}
```

### 3. 音频流播放：新建 StreamPlayer

**选择：新建 `audio.StreamPlayer`**

- 现有 `Player.Play()` 接受 `[]float32`，适合短音频（TTS）
- 音乐播放需要：HTTP 流式下载 → MP3 解码 → PCM → 播放，全程流式
- 新建 `StreamPlayer`，接受 URL，内部做流式解码和播放
- 复用 `go-mp3` 解码库（项目已有此依赖）

```go
type StreamPlayer struct {
    player   *Player      // 复用现有播放器
    cancel   context.CancelFunc
}

func (sp *StreamPlayer) Play(ctx context.Context, url string) error
func (sp *StreamPlayer) Stop()
```

### 4. 状态机：复用 StateSpeaking

**选择：复用 `StateSpeaking`**

- 从外部行为看，音乐播放和 TTS 播放没有区别（都是"正在播放音频，可被唤醒词打断"）
- 不新增状态，减少状态机复杂度
- Pipeline 内部通过一个标志位区分当前是 TTS 播放还是音乐播放

### 5. 工具实现流程

```
LLM 调用 play_music(keyword)
  → Tool 执行:
     1. TTS "正在为你搜索 xxx"
     2. music.Search(keyword)
     3. music.GetSongURL(songID)
     4. 返回结果给 LLM: "已找到 xxx，正在播放"
  → Pipeline 检测到音乐播放意图
     → StreamPlayer.Play(url)
```

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| NeteaseCloudMusicApi 随时可能失效 | 音乐 Provider 做接口抽象，后续可替换为其他源 |
| 部分歌曲需要 VIP | 搜索时尝试多个结果，优先选免费歌曲 |
| 用户未部署 API 服务 | 配置检查 + 友好错误提示 |
| 长时间音乐播放占用扬声器 | 唤醒词打断机制已有，无需额外处理 |

## Open Questions

- 是否需要支持"下一首"、"暂停"等播控指令？（建议第一版不做，后续迭代）
- 音乐播放音量是否需要和 TTS 音量独立控制？
- 是否考虑缓存最近播放过的歌曲 URL 以减少 API 调用？
