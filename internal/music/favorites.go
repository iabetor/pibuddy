package music

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FavoriteSong 收藏的歌曲信息。
type FavoriteSong struct {
	ID       int64  `json:"id"`
	MID      string `json:"mid,omitempty"`
	MediaMID string `json:"media_mid,omitempty"`
	Name     string `json:"name"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Provider string `json:"provider"` // netease 或 qq
	AddedAt  string `json:"added_at"`
}

// FavoritesList 用户收藏列表。
type FavoritesList struct {
	UserName  string         `json:"user_name"`
	Songs     []FavoriteSong `json:"songs"`
	UpdatedAt string         `json:"updated_at"`
}

// FavoritesStore 收藏存储管理器。
type FavoritesStore struct {
	dataDir string
	mu      sync.RWMutex
}

// NewFavoritesStore 创建收藏存储。
func NewFavoritesStore(dataDir string) *FavoritesStore {
	return &FavoritesStore{
		dataDir: dataDir,
	}
}

// Add 添加歌曲到用户收藏。
func (s *FavoritesStore) Add(userName string, song FavoriteSong) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.load(userName)
	if err != nil {
		return err
	}

	// 检查是否已收藏
	for _, s := range list.Songs {
		if s.ID == song.ID && s.Provider == song.Provider {
			return fmt.Errorf("歌曲已在收藏列表中")
		}
	}

	song.AddedAt = time.Now().Format("2006-01-02 15:04:05")
	list.Songs = append(list.Songs, song)
	list.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")

	return s.save(list)
}

// Remove 从用户收藏中删除歌曲。
func (s *FavoritesStore) Remove(userName string, songID int64, provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.load(userName)
	if err != nil {
		return err
	}

	found := false
	newSongs := make([]FavoriteSong, 0, len(list.Songs))
	for _, s := range list.Songs {
		if s.ID == songID && s.Provider == provider {
			found = true
			continue
		}
		newSongs = append(newSongs, s)
	}

	if !found {
		return fmt.Errorf("歌曲不在收藏列表中")
	}

	list.Songs = newSongs
	list.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")

	return s.save(list)
}

// List 获取用户收藏列表。
func (s *FavoritesStore) List(userName string) ([]FavoriteSong, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list, err := s.load(userName)
	if err != nil {
		return nil, err
	}

	return list.Songs, nil
}

// Clear 清空用户收藏。
func (s *FavoritesStore) Clear(userName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	list := &FavoritesList{
		UserName:  userName,
		Songs:     []FavoriteSong{},
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	return s.save(list)
}

// load 加载用户收藏列表。
func (s *FavoritesStore) load(userName string) (*FavoritesList, error) {
	filePath := s.getFilePath(userName)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，返回空列表
			return &FavoritesList{
				UserName: userName,
				Songs:    []FavoriteSong{},
			}, nil
		}
		return nil, fmt.Errorf("读取收藏文件失败: %w", err)
	}

	var list FavoritesList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("解析收藏文件失败: %w", err)
	}

	return &list, nil
}

// save 保存用户收藏列表。
func (s *FavoritesStore) save(list *FavoritesList) error {
	// 确保目录存在
	dir := filepath.Dir(s.getFilePath(list.UserName))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化收藏列表失败: %w", err)
	}

	filePath := s.getFilePath(list.UserName)
	return os.WriteFile(filePath, data, 0644)
}

// getFilePath 获取用户收藏文件路径。
func (s *FavoritesStore) getFilePath(userName string) string {
	return filepath.Join(s.dataDir, "favorites", userName+".json")
}

// GetUserName 获取实际使用的用户名（未识别时返回 guest）。
func (s *FavoritesStore) GetUserName(identifiedName string) string {
	if identifiedName == "" {
		return "guest"
	}
	return identifiedName
}
