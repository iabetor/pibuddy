package music

import (
	"sync"
	"time"
)

// PausedMusicInfo 暂停的音乐信息。
type PausedMusicInfo struct {
	Items       []PlaylistItem
	Index       int
	Mode        PlayMode
	SongName    string
	PositionSec float64   // 已播放的秒数
	PausedAt    time.Time // 打断时的时间
	CacheKey    string    // 缓存 key（用于判断是否可恢复位置）
}

// PausedMusicStore 暂停音乐状态存储。
type PausedMusicStore struct {
	mu     sync.RWMutex
	paused *PausedMusicInfo
}

// NewPausedMusicStore 创建暂停音乐状态存储。
func NewPausedMusicStore() *PausedMusicStore {
	return &PausedMusicStore{}
}

// Save 保存暂停状态。
func (s *PausedMusicStore) Save(items []PlaylistItem, index int, mode PlayMode, songName string, positionSec float64, cacheKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.paused = &PausedMusicInfo{
		Items:       items,
		Index:       index,
		Mode:        mode,
		SongName:    songName,
		PositionSec: positionSec,
		PausedAt:    time.Now(),
		CacheKey:    cacheKey,
	}
}

// Get 获取暂停状态。
func (s *PausedMusicStore) Get() *PausedMusicInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.paused == nil {
		return nil
	}

	// 返回副本，避免外部修改
	info := &PausedMusicInfo{
		Items:       make([]PlaylistItem, len(s.paused.Items)),
		Index:       s.paused.Index,
		Mode:        s.paused.Mode,
		SongName:    s.paused.SongName,
		PositionSec: s.paused.PositionSec,
		PausedAt:    s.paused.PausedAt,
		CacheKey:    s.paused.CacheKey,
	}
	copy(info.Items, s.paused.Items)
	return info
}

// Clear 清除暂停状态。
func (s *PausedMusicStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.paused = nil
}

// HasPaused 是否有暂停的音乐。
func (s *PausedMusicStore) HasPaused() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.paused != nil && len(s.paused.Items) > 0
}
