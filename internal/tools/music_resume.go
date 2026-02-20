package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/iabetor/pibuddy/internal/audio"
	"github.com/iabetor/pibuddy/internal/music"
)

// ResumeMusicTool 恢复播放工具。
type ResumeMusicTool struct {
	playlist    *music.Playlist
	pausedStore *music.PausedMusicStore
	musicCache  *audio.MusicCache
}

// NewResumeMusicTool 创建恢复播放工具。
func NewResumeMusicTool(playlist *music.Playlist, pausedStore *music.PausedMusicStore, musicCache *audio.MusicCache) *ResumeMusicTool {
	return &ResumeMusicTool{
		playlist:    playlist,
		pausedStore: pausedStore,
		musicCache:  musicCache,
	}
}

// Name 返回工具名称。
func (t *ResumeMusicTool) Name() string {
	return "resume_music"
}

// Description 返回工具描述。
func (t *ResumeMusicTool) Description() string {
	return `恢复之前被打断的音乐播放。当音乐被唤醒词打断后，可以说"继续播放"恢复。如果打断超过一分钟，将从开头播放。`
}

// Parameters 返回工具参数定义。
func (t *ResumeMusicTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

// Execute 执行工具。
func (t *ResumeMusicTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	paused := t.pausedStore.Get()
	if paused == nil || len(paused.Items) == 0 {
		result := MusicResult{
			Success: false,
			Error:   "没有暂停的音乐",
		}
		return marshalResult(result)
	}

	// 检查时间间隔：超过 1 分钟就不恢复位置
	timeSincePaused := time.Since(paused.PausedAt)
	fromPosition := timeSincePaused <= time.Minute

	// 恢复播放列表和当前索引
	t.playlist.ReplaceWithIndex(paused.Items, paused.Index)
	t.playlist.SetMode(paused.Mode)

	// 获取当前歌曲
	item := t.playlist.Current()
	if item == nil {
		result := MusicResult{
			Success: false,
			Error:   "无法获取当前歌曲",
		}
		return marshalResult(result)
	}

	// 清除暂停状态
	t.pausedStore.Clear()

	// 返回当前歌曲的信息
	result := MusicResult{
		Success:      true,
		SongName:     item.Song.Name,
		Artist:       item.Song.Artist,
		URL:          item.URL,
		CacheKey:     item.CacheKey,
		PlaylistSize: len(paused.Items),
	}

	// 检查是否可以从位置恢复
	if fromPosition && paused.PositionSec > 0 && paused.CacheKey != "" && t.musicCache != nil {
		if _, ok := t.musicCache.Lookup(paused.CacheKey); ok {
			// 缓存存在，返回位置信息让 Pipeline 处理
			result.PositionSec = paused.PositionSec
			result.Message = fmt.Sprintf("从 %.0f 秒处恢复播放", paused.PositionSec)
		} else {
			result.Message = "缓存不存在，从头播放"
		}
	} else if timeSincePaused > time.Minute {
		result.Message = fmt.Sprintf("暂停已超过 %.0f 分钟，从头播放", timeSincePaused.Minutes())
	}

	return marshalResult(result)
}

// StopMusicTool 停止播放工具（清除暂停状态）。
type StopMusicTool struct {
	playlist    *music.Playlist
	pausedStore *music.PausedMusicStore
}

// NewStopMusicTool 创建停止播放工具。
func NewStopMusicTool(playlist *music.Playlist, pausedStore *music.PausedMusicStore) *StopMusicTool {
	return &StopMusicTool{
		playlist:    playlist,
		pausedStore: pausedStore,
	}
}

// Name 返回工具名称。
func (t *StopMusicTool) Name() string {
	return "stop_music"
}

// Description 返回工具描述。
func (t *StopMusicTool) Description() string {
	return `停止播放音乐。与暂停不同，停止后无法通过"继续播放"恢复。`
}

// Parameters 返回工具参数定义。
func (t *StopMusicTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

// Execute 执行工具。
func (t *StopMusicTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	// 清空播放列表
	t.playlist.Clear()

	// 清除暂停状态
	t.pausedStore.Clear()

	result := MusicResult{
		Success: true,
	}
	return marshalResult(result)
}
