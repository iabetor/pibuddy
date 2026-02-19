package rss

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
)

// FeedStore 订阅源持久化存储。
type FeedStore struct {
	mu       sync.RWMutex
	filePath string
	feeds    []Feed
}

// NewFeedStore 创建订阅源存储。
func NewFeedStore(dataDir string) (*FeedStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	s := &FeedStore{
		filePath: filepath.Join(dataDir, "rss_feeds.json"),
	}
	if err := s.load(); err != nil {
		logger.Warnf("[rss] 加载订阅源数据失败（将使用空列表）: %v", err)
		s.feeds = make([]Feed, 0)
	}
	return s, nil
}

func (s *FeedStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.feeds = make([]Feed, 0)
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &s.feeds)
}

func (s *FeedStore) save() error {
	data, err := json.MarshalIndent(s.feeds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// Add 添加订阅源。如果 URL 已存在则返回错误。
func (s *FeedStore) Add(feed Feed) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, f := range s.feeds {
		if f.URL == feed.URL {
			return fmt.Errorf("该订阅源已存在: %s", f.Name)
		}
	}

	if feed.ID == "" {
		feed.ID = fmt.Sprintf("rss_%d", time.Now().UnixMilli())
	}
	if feed.AddedAt.IsZero() {
		feed.AddedAt = time.Now()
	}

	s.feeds = append(s.feeds, feed)
	return s.save()
}

// List 列出所有订阅源。
func (s *FeedStore) List() []Feed {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Feed, len(s.feeds))
	copy(result, s.feeds)
	return result
}

// Delete 根据 ID 或名称删除订阅源。
func (s *FeedStore) Delete(idOrName string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	lower := strings.ToLower(idOrName)
	for i, f := range s.feeds {
		if f.ID == idOrName || strings.ToLower(f.Name) == lower {
			s.feeds = append(s.feeds[:i], s.feeds[i+1:]...)
			_ = s.save()
			return true
		}
	}
	return false
}

// FindByName 按名称模糊查找订阅源。
func (s *FeedStore) FindByName(name string) *Feed {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lower := strings.ToLower(name)
	for _, f := range s.feeds {
		if strings.Contains(strings.ToLower(f.Name), lower) {
			result := f
			return &result
		}
	}
	return nil
}

// UpdateLastFetched 更新订阅源的最后抓取时间。
func (s *FeedStore) UpdateLastFetched(id string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.feeds {
		if s.feeds[i].ID == id {
			s.feeds[i].LastFetched = t
			_ = s.save()
			return
		}
	}
}
