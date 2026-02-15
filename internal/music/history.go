package music

import (
	"encoding/json"
	"github.com/iabetor/pibuddy/internal/logger"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HistoryEntry 播放历史条目。
type HistoryEntry struct {
	ID        int64  `json:"id"`        // 歌曲ID
	Name      string `json:"name"`      // 歌曲名
	Artist    string `json:"artist"`    // 歌手名
	Album     string `json:"album"`     // 专辑名
	PlayedAt  string `json:"played_at"` // 播放时间
	PlayCount int    `json:"play_count"`// 播放次数
}

// HistoryStore 播放历史持久化存储。
type HistoryStore struct {
	mu       sync.RWMutex
	filePath string
	entries  []HistoryEntry
	maxSize  int // 最大历史记录数
}

// NewHistoryStore 创建播放历史存储。
func NewHistoryStore(dataDir string) (*HistoryStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	s := &HistoryStore{
		filePath: filepath.Join(dataDir, "music_history.json"),
		maxSize:  100, // 默认保留最近100首
	}
	if err := s.load(); err != nil {
		logger.Warnf("[music] 加载播放历史失败（将使用空列表）: %v", err)
		s.entries = make([]HistoryEntry, 0)
	}
	return s, nil
}

func (s *HistoryStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.entries = make([]HistoryEntry, 0)
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &s.entries)
}

func (s *HistoryStore) save() error {
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// Add 添加或更新播放记录。
func (s *HistoryStore) Add(song Song) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Format("2006-01-02 15:04:05")

	// 检查是否已存在
	for i := range s.entries {
		if s.entries[i].ID == song.ID {
			s.entries[i].PlayCount++
			s.entries[i].PlayedAt = now
			// 移到最前面
			entry := s.entries[i]
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			s.entries = append([]HistoryEntry{entry}, s.entries...)
			return s.save()
		}
	}

	// 新增记录
	entry := HistoryEntry{
		ID:        song.ID,
		Name:      song.Name,
		Artist:    song.Artist,
		Album:     song.Album,
		PlayedAt:  now,
		PlayCount: 1,
	}
	s.entries = append([]HistoryEntry{entry}, s.entries...)

	// 限制最大数量
	if len(s.entries) > s.maxSize {
		s.entries = s.entries[:s.maxSize]
	}

	return s.save()
}

// List 获取播放历史列表。
func (s *HistoryStore) List(limit int) []HistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.entries) {
		limit = len(s.entries)
	}

	result := make([]HistoryEntry, limit)
	copy(result, s.entries[:limit])
	return result
}

// Clear 清空播放历史。
func (s *HistoryStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make([]HistoryEntry, 0)
	return s.save()
}
