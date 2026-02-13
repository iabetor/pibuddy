## Context

PiBuddy 当前是纯对话型语音助手，所有语音输入经 ASR → LLM → TTS 播放文本回复。
需要扩展为支持"技能分发"的架构，第一个技能是音乐播放。

核心挑战：
- 当前 `processQuery` 是一条直通管线（query → LLM → TTS），需要在 LLM 环节加入意图识别
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
- 不做多音源聚合（只接一个 API 后端）
- 不做歌词显示
- 不做 QQ 音乐账号登录流程（Cookie 手动配置）

## Decisions

### 1. 意图识别：LLM Function Calling vs 关键词匹配

**选择：LLM 自然语言判断**

让 LLM 在 system prompt 中约定输出格式。当用户意图是播放音乐时，LLM 回复特定 JSON 格式（如 `{"action":"play_music","query":"小星星"}`），否则正常回复文本。

- 优点：灵活，能理解"来首歌"、"放点音乐"、"我想听 xxx" 等多种表达
- 缺点：依赖 LLM 稳定输出格式，需要在 prompt 中严格约定
- 备选：先用简单关键词匹配（"播放"、"放歌"）做 MVP，后续切 LLM

### 2. 音乐 API：Go 直接调用 vs 外部服务

**选择：Go 原生 HTTP 客户端直接调用 QQ 音乐 Web 接口**

- 不依赖外部 Node.js/Python 服务
- 参考 Meting / musicapi 的接口逆向逻辑，用 Go 重写核心搜索和 URL 获取
- 封装为 `internal/music/` 包，接口设计：

```go
type Provider interface {
    Search(ctx context.Context, keyword string) ([]Song, error)
    GetStreamURL(ctx context.Context, songID string) (string, error)
}
```

### 3. 音频流播放：复用 Player vs 新建 StreamPlayer

**选择：新建 `audio.StreamPlayer`**

- 现有 `Player.Play()` 接受 `[]float32`，适合短音频（TTS）
- 音乐播放需要：HTTP 流式下载 → MP3 解码 → PCM → 播放，全程流式，不能一次性加载到内存
- 新建 `StreamPlayer`，接受 `io.Reader`（HTTP response body），内部做流式解码和播放
- 复用 `go-mp3` 解码库（项目已有此依赖）

### 4. 状态机：新增状态 vs 复用 StateSpeaking

**选择：复用 `StateSpeaking`**

- 从外部行为看，音乐播放和 TTS 播放没有区别（都是"正在播放音频，可被唤醒词打断"）
- 不新增状态，减少状态机复杂度
- Pipeline 内部通过一个标志位区分当前是 TTS 播放还是音乐播放

### 5. processQuery 改造

当前流程：
```
ASR text → LLM stream → 按句 TTS → 播放
```

改造后：
```
ASR text → LLM（判断意图）
  ├─ 意图=chat → 现有 TTS 流程（不变）
  └─ 意图=play_music
       → 提取歌名
       → TTS "正在为你搜索 xxx"
       → music.Search(keyword)
       → music.GetStreamURL(songID)
       → StreamPlayer.Play(url)
```

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| QQ 音乐 Web 接口随时可能变更或封禁 | 音乐 Provider 做接口抽象，后续可替换为其他源（网易云、咪咕等） |
| VIP 歌曲需要 Cookie，Cookie 会过期 | 配置文件中放 Cookie，过期后搜索降级到免费歌曲 |
| LLM 意图判断不稳定 | 回复格式校验 + 降级为纯文本对话 |
| 长时间音乐播放占用扬声器 | 唤醒词打断机制已有，无需额外处理 |

## Open Questions

- 是否需要支持"下一首"、"暂停"等播控指令？（建议第一版不做，后续迭代）
- 音乐播放音量是否需要和 TTS 音量独立控制？
- 是否考虑缓存最近播放过的歌曲 URL 以减少 API 调用？
