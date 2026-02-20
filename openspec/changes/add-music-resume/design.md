## Context

PiBuddy 的音乐播放会在用户唤醒打断时完全停止，导致播放状态丢失。用户说"继续播放"时会重新搜索，可能播放到不同的歌曲版本。

**现有系统**：
- `audio.StreamPlayer` - 流式播放器，只有 `Stop()` 方法
- `music.Playlist` - 播放列表管理，有 `Current()` 和 `CurrentIndex()` 方法
- `Pipeline.interruptSpeak()` - 打断时调用 `streamPlayer.Stop()`

**约束**：
- 需要保留播放位置（可选）
- 需要保留播放列表和当前索引
- 歌曲URL可能过期，需要处理

## Goals / Non-Goals

**Goals**：
- 唤醒打断时暂停播放而非停止
- 保留播放状态（列表、索引、模式）
- 提供"继续播放"功能恢复播放
- 处理歌曲URL过期的情况

**Non-Goals**：
- 不保存精确的播放位置（秒级）
- 不支持多级暂停历史
- 不区分不同用户的暂停状态

## Decisions

### 1. 暂停 vs 停止

**选择**：区分 `Pause()` 和 `Stop()`

| 方法 | 行为 | 可恢复 |
|------|------|--------|
| `Pause()` | 停止播放，保留状态 | ✅ |
| `Stop()` | 停止播放，清除状态 | ❌ |

**触发场景**：
- 唤醒打断 → `Pause()`
- 用户说"停止播放" → `Stop()`

### 2. 状态保存

**选择**：在 Pipeline 中保存暂停状态

```go
type PausedMusicInfo struct {
    Items    []music.PlaylistItem  // 播放列表快照
    Index    int                   // 当前索引
    Mode     music.PlayMode        // 播放模式
    SongInfo music.Song            // 当前歌曲信息（用于日志）
}

type Pipeline struct {
    // ...
    pausedMusic     *PausedMusicInfo
    pausedMusicMu   sync.RWMutex
}
```

**保存时机**：在 `interruptSpeak()` 中，检查是否有音乐正在播放，如果有则保存。

### 3. 恢复逻辑

```go
func (t *ResumeMusicTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    // 1. 检查是否有暂停的音乐
    if t.pausedMusic == nil {
        return "没有暂停的音乐", nil
    }

    // 2. 恢复播放列表
    t.playlist.Replace(t.pausedMusic.Items)
    t.playlist.SetMode(t.pausedMusic.Mode)

    // 3. 恢复播放位置
    // 方案A：从头开始（简单）
    // 方案B：从暂停位置继续（需要保存播放位置）

    // 4. 开始播放
    // ...
}
```

### 4. 处理URL过期

**问题**：歌曲URL通常有时效性，暂停时间过长可能导致URL失效。

**解决方案**：
1. 恢复时检测播放是否失败
2. 如果失败，重新从 Provider 获取 URL
3. 如果仍失败，跳过该歌曲播放下一首

```go
// 在恢复播放时
url, err := getOrRefreshURL(song)
if err != nil {
    // URL过期，尝试重新获取
    url, err = t.provider.GetSongURL(ctx, song.ID)
    if err != nil {
        // 跳过此歌
        return t.playNext()
    }
}
```

### 5. 工具参数设计

```go
// ResumeMusicTool - 恢复播放
// Parameters: {}
// 无参数，自动恢复暂停的音乐

// 可选扩展：指定从第几首开始
type ResumeMusicArgs struct {
    StartFrom int `json:"start_from"` // 从第几首开始（默认从暂停位置）
}
```

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| 暂停状态占用内存 | 播放列表通常不超过50首，可接受 |
| URL过期 | 恢复时重新获取URL |
| 多次打断状态覆盖 | 只保留最后一次暂停状态 |
| 播放完毕后仍有暂停状态 | 播放完毕时清除暂停状态 |

## Migration Plan

无需迁移，纯新增功能。但需要注意：
- 现有的 `Stop()` 行为保持不变
- 只有唤醒打断会触发 `Pause()`

## Open Questions

- [ ] 是否需要保存精确的播放位置（秒级）？这需要 StreamPlayer 支持获取当前播放位置
- [ ] "暂停音乐"指令是否应该也保存恢复状态？当前设计是不保存
- [ ] 是否需要在系统提示词中添加"继续播放"的说明？
