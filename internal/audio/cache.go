package audio

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/iabetor/pibuddy/internal/database"
	"github.com/iabetor/pibuddy/internal/logger"
)

// CacheEntry 缓存索引中的一条记录。
type CacheEntry struct {
	ID          int64
	Name        string
	Artist      string
	Album       string
	Provider    string
	ProviderID  int64
	Duration    int64  // 时长（秒）
	Size        int64  // 文件大小（字节）
	PlayCount   int64  // 播放次数
	CachedAt    string
	LastPlayed  string
}

// MusicCache 管理音乐文件缓存和索引（SQLite 版本）。
type MusicCache struct {
	mu       sync.RWMutex
	db       *database.DB
	cacheDir string
	maxSize  int64 // 最大缓存大小（字节），0 表示禁用缓存
}

// NewMusicCache 创建音乐缓存管理器。
func NewMusicCache(db *database.DB, cacheDir string, maxSizeMB int64) (*MusicCache, error) {
	if maxSizeMB == 0 {
		return &MusicCache{
			db:       db,
			cacheDir: cacheDir,
			maxSize:  0,
		}, nil
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	mc := &MusicCache{
		db:       db,
		cacheDir: cacheDir,
		maxSize:  maxSizeMB * 1024 * 1024,
	}

	// 校验索引：移除本地文件不存在的条目
	mc.validateIndex()

	logger.Infof("[cache] 音乐缓存已初始化: 目录 %s, 最大 %dMB", cacheDir, maxSizeMB)
	return mc, nil
}

// Enabled 返回缓存是否启用。
func (mc *MusicCache) Enabled() bool {
	return mc.maxSize > 0
}

// CacheDir 返回缓存目录路径。
func (mc *MusicCache) CacheDir() string {
	return mc.cacheDir
}

// FilePath 返回缓存文件的完整路径。
func (mc *MusicCache) FilePath(cacheKey string) string {
	return filepath.Join(mc.cacheDir, cacheKey+".mp3")
}

// TempFilePath 返回缓存临时文件的完整路径。
func (mc *MusicCache) TempFilePath(cacheKey string) string {
	return filepath.Join(mc.cacheDir, cacheKey+".mp3.tmp")
}

// Lookup 查找缓存条目，返回本地文件路径和是否命中。
func (mc *MusicCache) Lookup(cacheKey string) (string, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var entry CacheEntry
	err := mc.db.QueryRow(`
		SELECT id, name, artist, album, provider, provider_id, duration, size, play_count, cached_at, last_played
		FROM music_cache WHERE cache_key = ?
	`, cacheKey).Scan(&entry.ID, &entry.Name, &entry.Artist, &entry.Album, &entry.Provider,
		&entry.ProviderID, &entry.Duration, &entry.Size, &entry.PlayCount, &entry.CachedAt, &entry.LastPlayed)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", false
		}
		return "", false
	}

	filePath := mc.FilePath(cacheKey)
	if _, err := os.Stat(filePath); err != nil {
		return "", false
	}

	// 更新 last_played 和 play_count（异步）
	go func() {
		mc.db.Exec(`UPDATE music_cache SET last_played = ?, play_count = play_count + 1 WHERE cache_key = ?`,
			time.Now().Format(time.RFC3339), cacheKey)
	}()

	return filePath, true
}

// TouchLastPlayed 更新缓存条目的最后播放时间。
func (mc *MusicCache) TouchLastPlayed(cacheKey string) {
	mc.db.Exec(`UPDATE music_cache SET last_played = ?, play_count = play_count + 1 WHERE cache_key = ?`,
		time.Now().Format(time.RFC3339), cacheKey)
}

// Search 按关键词模糊搜索缓存索引（name/artist 匹配）。
func (mc *MusicCache) Search(keyword string) []CacheEntry {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	keyword = strings.ToLower(keyword)
	keywords := strings.Fields(keyword)

	rows, err := mc.db.Query(`
		SELECT id, name, artist, album, provider, provider_id, duration, size, play_count, cached_at, last_played
		FROM music_cache
		WHERE LOWER(name) LIKE ? OR LOWER(artist) LIKE ?
		ORDER BY last_played DESC
	`, "%"+keyword+"%", "%"+keyword+"%")
	if err != nil {
		return nil
	}
	defer rows.Close()

	type scoredEntry struct {
		entry CacheEntry
		score int
	}
	var results []scoredEntry

	for rows.Next() {
		var entry CacheEntry
		if err := rows.Scan(&entry.ID, &entry.Name, &entry.Artist, &entry.Album, &entry.Provider,
			&entry.ProviderID, &entry.Duration, &entry.Size, &entry.PlayCount, &entry.CachedAt, &entry.LastPlayed); err != nil {
			continue
		}

		// 检查文件是否存在
		cacheKey := fmt.Sprintf("%s_%d", entry.Provider, entry.ProviderID)
		filePath := mc.FilePath(cacheKey)
		if _, err := os.Stat(filePath); err != nil {
			continue
		}

		// 计算匹配分数
		nameLower := strings.ToLower(entry.Name)
		artistLower := strings.ToLower(entry.Artist)

		score := 0
		if nameLower == keyword {
			score += 10
		} else if strings.Contains(nameLower, keyword) {
			score += 5
		}
		if strings.Contains(artistLower, keyword) {
			score += 2
		}

		// 多关键词匹配
		if len(keywords) > 1 {
			for _, kw := range keywords {
				if len(kw) >= 2 && strings.Contains(nameLower, kw) {
					score += 3
					break
				}
			}
		}

		results = append(results, scoredEntry{entry: entry, score: score})
	}

	// 按分数排序
	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return results[i].entry.LastPlayed > results[j].entry.LastPlayed
	})

	entries := make([]CacheEntry, len(results))
	for i, r := range results {
		entries[i] = r.entry
	}
	return entries
}

// Store 将歌曲信息写入缓存索引。
func (mc *MusicCache) Store(cacheKey string, entry CacheEntry) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now().Format(time.RFC3339)

	// 获取文件大小
	filePath := mc.FilePath(cacheKey)
	if info, err := os.Stat(filePath); err == nil {
		entry.Size = info.Size()
	}

	// 解析 cacheKey 获取 provider 和 provider_id
	// cacheKey 格式: provider_id

	_, err := mc.db.Exec(`
		INSERT OR REPLACE INTO music_cache
		(cache_key, name, artist, album, provider, provider_id, duration, size, play_count, cached_at, last_played)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)
	`, cacheKey, entry.Name, entry.Artist, entry.Album, entry.Provider, entry.ProviderID,
		entry.Duration, entry.Size, now, now)

	if err != nil {
		return fmt.Errorf("保存缓存索引失败: %w", err)
	}

	// 检查并淘汰
	mc.evictLocked()

	logger.Infof("[cache] 已缓存: %s - %s (%s, %d bytes)", entry.Name, entry.Artist, cacheKey, entry.Size)
	return nil
}

// List 返回所有缓存条目，按 last_played 倒序排列。
func (mc *MusicCache) List() []CacheEntry {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	rows, err := mc.db.Query(`
		SELECT id, name, artist, album, provider, provider_id, duration, size, play_count, cached_at, last_played
		FROM music_cache
		ORDER BY last_played DESC
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []CacheEntry
	for rows.Next() {
		var entry CacheEntry
		if err := rows.Scan(&entry.ID, &entry.Name, &entry.Artist, &entry.Album, &entry.Provider,
			&entry.ProviderID, &entry.Duration, &entry.Size, &entry.PlayCount, &entry.CachedAt, &entry.LastPlayed); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// Delete 删除指定缓存条目（按关键词匹配 name/artist）。
func (mc *MusicCache) Delete(keyword string, excludeArtists []string) int {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	keyword = strings.ToLower(keyword)
	excludeSet := make(map[string]bool)
	for _, a := range excludeArtists {
		excludeSet[strings.ToLower(a)] = true
	}

	rows, err := mc.db.Query(`
		SELECT cache_key, name, artist FROM music_cache
		WHERE LOWER(name) LIKE ? OR LOWER(artist) LIKE ?
	`, "%"+keyword+"%", "%"+keyword+"%")
	if err != nil {
		return 0
	}
	defer rows.Close()

	deleted := 0
	for rows.Next() {
		var cacheKey, name, artist string
		if err := rows.Scan(&cacheKey, &name, &artist); err != nil {
			continue
		}

		artistLower := strings.ToLower(artist)
		if excludeSet[artistLower] {
			continue
		}

		// 删除文件
		filePath := mc.FilePath(cacheKey)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			continue
		}

		// 删除数据库记录
		mc.db.Exec("DELETE FROM music_cache WHERE cache_key = ?", cacheKey)
		logger.Infof("[cache] 已删除缓存: %s - %s (%s)", name, artist, cacheKey)
		deleted++
	}

	return deleted
}

// DeleteByKey 删除指定 cacheKey 的缓存条目。
func (mc *MusicCache) DeleteByKey(cacheKey string) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	filePath := mc.FilePath(cacheKey)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return false
	}

	result, _ := mc.db.Exec("DELETE FROM music_cache WHERE cache_key = ?", cacheKey)
	affected, _ := result.RowsAffected()
	return affected > 0
}

// Stats 返回缓存统计信息。
func (mc *MusicCache) Stats() (count int, totalSize int64) {
	mc.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(size), 0) FROM music_cache").Scan(&count, &totalSize)
	return
}

// validateIndex 校验索引，移除本地文件不存在的条目。
func (mc *MusicCache) validateIndex() {
	rows, err := mc.db.Query("SELECT cache_key FROM music_cache")
	if err != nil {
		return
	}
	defer rows.Close()

	removed := 0
	for rows.Next() {
		var cacheKey string
		if err := rows.Scan(&cacheKey); err != nil {
			continue
		}

		filePath := mc.FilePath(cacheKey)
		if _, err := os.Stat(filePath); err != nil {
			mc.db.Exec("DELETE FROM music_cache WHERE cache_key = ?", cacheKey)
			removed++
		}
	}

	if removed > 0 {
		logger.Infof("[cache] 索引校验：移除 %d 个无效条目", removed)
	}

	var count int
	var totalSize int64
	mc.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(size), 0) FROM music_cache").Scan(&count, &totalSize)
	logger.Infof("[cache] 缓存已加载: %d 首歌曲, %.2f MB", count, float64(totalSize)/1024/1024)
}

// evictLocked 检查缓存总大小并淘汰最久未播放的。
func (mc *MusicCache) evictLocked() {
	if mc.maxSize <= 0 {
		return
	}

	var totalSize int64
	mc.db.QueryRow("SELECT COALESCE(SUM(size), 0) FROM music_cache").Scan(&totalSize)

	if totalSize <= mc.maxSize {
		return
	}

	// 按播放次数和最后播放时间淘汰
	rows, err := mc.db.Query(`
		SELECT cache_key, name, artist, size FROM music_cache
		ORDER BY play_count ASC, last_played ASC
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() && totalSize > mc.maxSize {
		var cacheKey, name, artist string
		var size int64
		if err := rows.Scan(&cacheKey, &name, &artist, &size); err != nil {
			continue
		}

		filePath := mc.FilePath(cacheKey)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			continue
		}

		mc.db.Exec("DELETE FROM music_cache WHERE cache_key = ?", cacheKey)
		totalSize -= size
		logger.Infof("[cache] LRU 淘汰: %s - %s (%s)", name, artist, cacheKey)
	}
}
