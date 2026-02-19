package music

import (
	"context"
	"fmt"
	"sync"

	"github.com/iabetor/pibuddy/internal/logger"
)

// PlayMode 播放模式。
type PlayMode int

const (
	PlayModeSequence PlayMode = iota // 顺序播放（到末尾停止）
	PlayModeLoop                     // 列表循环
	PlayModeSingle                   // 单曲循环
)

func (m PlayMode) String() string {
	switch m {
	case PlayModeSequence:
		return "顺序播放"
	case PlayModeLoop:
		return "列表循环"
	case PlayModeSingle:
		return "单曲循环"
	default:
		return "未知"
	}
}

// PlaylistItem 播放列表中的一项，包含歌曲信息和播放 URL。
type PlaylistItem struct {
	Song     Song
	URL      string // 播放地址（可能为空，需要时再获取）
	CacheKey string // 缓存标识，如 "qq_12345678"
}

// Playlist 播放列表管理器，支持队列管理、播放模式切换和自动下一首。
type Playlist struct {
	mu       sync.RWMutex
	items    []PlaylistItem
	current  int      // 当前播放索引，-1 表示未开始
	mode     PlayMode // 播放模式
	provider Provider // 用于懒加载 URL
	history  *HistoryStore
}

// NewPlaylist 创建播放列表。
func NewPlaylist(provider Provider, history *HistoryStore) *Playlist {
	return &Playlist{
		items:    make([]PlaylistItem, 0),
		current:  -1,
		mode:     PlayModeSequence,
		provider: provider,
		history:  history,
	}
}

// SetMode 设置播放模式。
func (pl *Playlist) SetMode(mode PlayMode) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.mode = mode
	logger.Infof("[playlist] 播放模式切换为: %s", mode)
}

// Mode 获取当前播放模式。
func (pl *Playlist) Mode() PlayMode {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	return pl.mode
}

// Add 向播放列表末尾添加歌曲。
func (pl *Playlist) Add(items ...PlaylistItem) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.items = append(pl.items, items...)
	logger.Debugf("[playlist] 添加 %d 首歌曲，列表共 %d 首", len(items), len(pl.items))
}

// Replace 替换整个播放列表并重置索引。
func (pl *Playlist) Replace(items []PlaylistItem) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.items = items
	pl.current = -1
	logger.Debugf("[playlist] 替换列表为 %d 首歌曲", len(items))
}

// Clear 清空播放列表。
func (pl *Playlist) Clear() {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.items = nil
	pl.current = -1
}

// Len 返回列表长度。
func (pl *Playlist) Len() int {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	return len(pl.items)
}

// Current 返回当前播放的歌曲信息，如果没有返回 nil。
func (pl *Playlist) Current() *PlaylistItem {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	if pl.current < 0 || pl.current >= len(pl.items) {
		return nil
	}
	item := pl.items[pl.current]
	return &item
}

// CurrentIndex 返回当前索引（从0开始，-1 表示未开始）。
func (pl *Playlist) CurrentIndex() int {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	return pl.current
}

// Next 获取下一首歌曲的 URL，根据播放模式决定行为。
// 返回 URL、歌曲名、歌手名、缓存标识和是否有下一首。
// 如果到达列表末尾且非循环模式，返回 ("", "", "", "", false)。
// 对于有 CacheKey 的歌曲（缓存命中），不需要 URL，直接返回。
func (pl *Playlist) Next(ctx context.Context) (url, songName, artist, cacheKey string, ok bool) {
	pl.mu.Lock()

	skipped := 0
	maxSkips := len(pl.items) // 最多跳过整个列表，防止死循环

	for {
		if len(pl.items) == 0 {
			pl.mu.Unlock()
			return "", "", "", "", false
		}

		nextIdx := pl.nextIndex()
		if nextIdx < 0 {
			pl.mu.Unlock()
			return "", "", "", "", false
		}

		pl.current = nextIdx
		item := &pl.items[pl.current]

		// 有缓存标识的歌曲不需要 URL，可直接从本地播放
		if item.CacheKey != "" {
			// 记录播放历史
			if pl.history != nil {
				if addErr := pl.history.Add(item.Song); addErr != nil {
					logger.Debugf("[playlist] 保存播放历史失败: %v", addErr)
				}
			}
			logger.Infof("[playlist] 播放第 %d/%d 首: %s - %s (缓存)", pl.current+1, len(pl.items), item.Song.Name, item.Song.Artist)
			pl.mu.Unlock()
			return item.URL, item.Song.Name, item.Song.Artist, item.CacheKey, true
		}

		// 如果 URL 为空，尝试获取
		if item.URL == "" {
			// 释放锁再做网络请求，避免持锁阻塞
			song := item.Song
			pl.mu.Unlock()

			resolvedURL, err := pl.resolveURL(ctx, song)

			pl.mu.Lock()
			if err != nil || resolvedURL == "" {
				logger.Warnf("[playlist] 获取歌曲 URL 失败: %s - %s: %v", song.Name, song.Artist, err)
				skipped++
				if skipped >= maxSkips {
					logger.Warnf("[playlist] 已跳过 %d 首歌曲，全部无法播放", skipped)
					pl.mu.Unlock()
					return "", "", "", "", false
				}
				// 跳过此曲，继续循环尝试下一首
				continue
			}
			item.URL = resolvedURL
		}

		// 记录播放历史
		if pl.history != nil {
			if addErr := pl.history.Add(item.Song); addErr != nil {
				logger.Debugf("[playlist] 保存播放历史失败: %v", addErr)
			}
		}

		logger.Infof("[playlist] 播放第 %d/%d 首: %s - %s", pl.current+1, len(pl.items), item.Song.Name, item.Song.Artist)
		pl.mu.Unlock()
		return item.URL, item.Song.Name, item.Song.Artist, item.CacheKey, true
	}
}

// Peek 预览下一首歌曲信息（不改变当前索引）。
func (pl *Playlist) Peek() *PlaylistItem {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	if len(pl.items) == 0 {
		return nil
	}

	nextIdx := pl.nextIndex()
	if nextIdx < 0 {
		return nil
	}

	item := pl.items[nextIdx]
	return &item
}

// HasNext 检查是否有下一首。
func (pl *Playlist) HasNext() bool {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	return pl.nextIndex() >= 0
}

// nextIndex 根据播放模式计算下一个索引（调用方需持有锁）。
// 返回 -1 表示没有下一首。
func (pl *Playlist) nextIndex() int {
	if len(pl.items) == 0 {
		return -1
	}

	switch pl.mode {
	case PlayModeSingle:
		// 单曲循环：始终返回当前索引
		if pl.current < 0 {
			return 0
		}
		return pl.current

	case PlayModeLoop:
		// 列表循环：到末尾回到开头
		if pl.current < 0 {
			return 0
		}
		return (pl.current + 1) % len(pl.items)

	default: // PlayModeSequence
		// 顺序播放：到末尾停止
		next := pl.current + 1
		if next >= len(pl.items) {
			return -1
		}
		return next
	}
}

// resolveURL 为歌曲获取播放 URL（此方法不加锁，调用方应在无锁状态下调用）。
func (pl *Playlist) resolveURL(ctx context.Context, song Song) (string, error) {
	if pl.provider == nil {
		return "", fmt.Errorf("provider not set")
	}

	// 优先使用 QQ Provider 的 MID 接口
	if qqProvider, ok := pl.provider.(QQProvider); ok {
		mid := song.GetMID()
		if mid != "" {
			return qqProvider.GetSongURLWithMID(ctx, song.ID, mid)
		}
	}

	return pl.provider.GetSongURL(ctx, song.ID)
}

// Info 返回播放列表的摘要信息。
func (pl *Playlist) Info() string {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	if len(pl.items) == 0 {
		return "播放列表为空"
	}

	cur := "未开始"
	if pl.current >= 0 && pl.current < len(pl.items) {
		item := pl.items[pl.current]
		cur = fmt.Sprintf("%s - %s", item.Song.Name, item.Song.Artist)
	}

	return fmt.Sprintf("播放列表: %d 首歌曲, 当前: %s (%d/%d), 模式: %s",
		len(pl.items), cur, pl.current+1, len(pl.items), pl.mode)
}
