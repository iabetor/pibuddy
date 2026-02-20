## Context

PiBuddy 已有音乐播放系统和声纹识别系统。本项目在现有基础上添加收藏歌单功能，实现个性化音乐体验。

**现有系统**：
- 音乐播放器：`music.Provider`、`music.Playlist`
- 声纹识别：`voiceprint.Manager`
- 播放历史：`music.HistoryStore`

**约束**：
- 收藏歌曲需要能获取当前播放歌曲信息
- 需要知道当前说话用户（声纹识别结果）
- 存储需要持久化

## Goals / Non-Goals

**Goals**：
- 用户可以收藏当前播放的歌曲
- 不同用户有独立的收藏歌单
- 支持随机/顺序播放收藏
- 持久化存储

**Non-Goals**：
- 不实现云端同步
- 不实现歌单分享
- 不实现收藏歌曲的在线推荐

## Decisions

### 1. 存储方案

**选择**：JSON 文件存储，每个用户一个文件

**原因**：
- 简单可靠，无需数据库
- 与现有 HistoryStore 风格一致
- 方便调试和手动编辑

**替代方案**：
- SQLite（更复杂，不需要）
- 单文件存储所有用户（文件变大后性能差）

### 2. 用户识别

**选择**：通过 VoiceprintManager 获取当前用户

**逻辑**：
```go
// 在工具执行时获取当前用户
func (t *Tool) getCurrentUser() string {
    if t.voiceprintMgr != nil {
        if name := t.voiceprintMgr.GetCurrentSpeaker(); name != "" {
            return name
        }
    }
    return "guest" // 默认用户
}
```

### 3. 当前播放歌曲获取

**选择**：通过 Playlist 获取当前播放歌曲

```go
// 在工具执行时获取当前歌曲
func (t *AddFavoriteTool) getCurrentSong() *SongInfo {
    if t.playlist != nil {
        return t.playlist.GetCurrentSong()
    }
    return nil
}
```

**问题**：需要确认 Playlist 是否有 GetCurrentSong() 方法，可能需要扩展。

### 4. 播放模式

**随机播放**：
```go
func ShuffleSongs(songs []SongInfo) []SongInfo {
    result := make([]SongInfo, len(songs))
    copy(result, songs)
    rand.Shuffle(len(result), func(i, j int) {
        result[i], result[j] = result[j], result[i]
    })
    return result
}
```

**顺序播放**：按 `added_at` 时间排序

### 5. 工具参数设计

```go
// AddFavoriteTool - 无参数，收藏当前歌曲
// Parameters: {}

// PlayFavoritesTool
type PlayFavoritesArgs struct {
    Mode string `json:"mode"` // "random" 或 "sequential"，默认 "random"
}

// ListFavoritesTool - 无参数
// Parameters: {}

// RemoveFavoriteTool
type RemoveFavoriteArgs struct {
    SongID string `json:"song_id"` // 为空则删除当前播放歌曲
}
```

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| Playlist 没有 GetCurrentSong() | 扩展 Playlist 或使用 HistoryStore 获取最近播放 |
| 声纹识别不准确 | 用户可通过指令"XXX的收藏"指定用户 |
| 收藏歌曲下架 | 播放时检测失败跳过 |
| 并发写入冲突 | 使用文件锁或串行化写入 |

## Migration Plan

无需迁移，纯新增功能。

## Open Questions

- [ ] Playlist 是否有获取当前播放歌曲的方法？需要查看现有代码
- [ ] VoiceprintManager 是否有 GetCurrentSpeaker() 方法？
- [ ] 是否需要"播放XXX的收藏"功能（主人播放家人的收藏）？
