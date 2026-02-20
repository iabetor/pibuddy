package music

import "sync"

// PausedMusicInfo 暂停的音乐信息。
type PausedMusicInfo struct {
	Items    []PlaylistItem
	Index    int
	Mode     PlayMode
	SongName string
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
func (s *PausedMusicStore) Save(items []PlaylistItem, index int, mode PlayMode, songName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.paused = &PausedMusicInfo{
		Items:    items,
		Index:    index,
		Mode:     mode,
		SongName: songName,
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
		Items:    make([]PlaylistItem, len(s.paused.Items)),
		Index:    s.paused.Index,
		Mode:     s.paused.Mode,
		SongName: s.paused.SongName,
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
