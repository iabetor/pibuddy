package audio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
)

// CacheEntry 缓存索引中的一条记录。
type CacheEntry struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	Provider   string `json:"provider"`
	Size       int64  `json:"size"`
	CachedAt   string `json:"cached_at"`
	LastPlayed string `json:"last_played"`
}

// MusicCache 管理音乐文件缓存和索引。
type MusicCache struct {
	mu       sync.RWMutex
	cacheDir string
	maxSize  int64 // 最大缓存大小（字节），0 表示禁用缓存
	index    map[string]*CacheEntry
}

// NewMusicCache 创建音乐缓存管理器。
// cacheDir 为缓存目录路径，maxSizeMB 为最大缓存大小（MB），0 表示禁用缓存。
func NewMusicCache(cacheDir string, maxSizeMB int64) (*MusicCache, error) {
	if maxSizeMB == 0 {
		// 缓存被禁用
		return &MusicCache{
			cacheDir: cacheDir,
			maxSize:  0,
			index:    make(map[string]*CacheEntry),
		}, nil
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	mc := &MusicCache{
		cacheDir: cacheDir,
		maxSize:  maxSizeMB * 1024 * 1024,
		index:    make(map[string]*CacheEntry),
	}

	if err := mc.loadIndex(); err != nil {
		logger.Warnf("[cache] 加载缓存索引失败（将使用空索引）: %v", err)
	}

	// 校验索引：移除本地文件不存在的条目
	mc.validateIndex()

	return mc, nil
}

// Enabled 返回缓存是否启用。
func (mc *MusicCache) Enabled() bool {
	return mc.maxSize > 0
}

// Lookup 查找缓存条目，返回本地文件路径和是否命中。
func (mc *MusicCache) Lookup(cacheKey string) (string, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, ok := mc.index[cacheKey]
	if !ok {
		return "", false
	}

	filePath := filepath.Join(mc.cacheDir, cacheKey+".mp3")
	if _, err := os.Stat(filePath); err != nil {
		return "", false
	}

	// 更新 last_played（需要写锁，延迟更新）
	entry.LastPlayed = time.Now().Format(time.RFC3339)

	return filePath, true
}

// TouchLastPlayed 更新缓存条目的最后播放时间并持久化。
func (mc *MusicCache) TouchLastPlayed(cacheKey string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if entry, ok := mc.index[cacheKey]; ok {
		entry.LastPlayed = time.Now().Format(time.RFC3339)
		mc.saveIndexLocked()
	}
}

// Search 按关键词模糊搜索缓存索引（name/artist 匹配）。
// 返回匹配的缓存条目，优先按匹配度排序：
// - 歌名完全匹配 > 歌名包含关键词 > 歌手匹配（需同时有歌名部分匹配）
// - 同等匹配度时按 last_played 倒序排列。
func (mc *MusicCache) Search(keyword string) []CacheEntry {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	keyword = strings.ToLower(keyword)
	// 将关键词拆分为多个词（用于更精确的匹配）
	keywords := strings.Fields(keyword)

	type scoredEntry struct {
		entry CacheEntry
		score int // 匹配分数，越高越优先
	}
	var results []scoredEntry

	for _, entry := range mc.index {
		nameLower := strings.ToLower(entry.Name)
		artistLower := strings.ToLower(entry.Artist)

		// 歌名匹配：关键词包含歌名，或歌名包含关键词
		nameExactMatch := nameLower == keyword
		nameMatch := strings.Contains(nameLower, keyword) || strings.Contains(keyword, nameLower)

		// 歌手匹配：歌手包含关键词，或关键词包含歌手
		artistMatch := strings.Contains(artistLower, keyword) || strings.Contains(keyword, artistLower)

		// 如果关键词是多个词，检查是否有词匹配歌名
		// 这样 "千里之外 周杰伦" 可以匹配 "千里之外"，但不会匹配 "青花瓷"
		if !nameMatch && len(keywords) > 1 {
			for _, kw := range keywords {
				if len(kw) >= 2 && strings.Contains(nameLower, kw) {
					nameMatch = true
					break
				}
			}
		}

		// 必须至少有歌名匹配，单纯歌手匹配不算（避免 "周杰伦" 匹配到所有周杰伦的歌）
		if !nameMatch {
			continue
		}

		// 确认文件存在
		cacheKey := fmt.Sprintf("%s_%d", entry.Provider, entry.ID)
		filePath := filepath.Join(mc.cacheDir, cacheKey+".mp3")
		if _, err := os.Stat(filePath); err != nil {
			continue
		}

		// 计算匹配分数
		// 完全匹配歌名 = 10，歌名包含关键词 = 5，歌手匹配 = 2
		score := 0
		if nameExactMatch {
			score += 10
		} else if nameMatch {
			score += 5
		}
		if artistMatch {
			score += 2
		}
		results = append(results, scoredEntry{entry: *entry, score: score})
	}

	// 按匹配分数倒序，同分按 last_played 倒序
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
// filePath 是已写入的缓存文件路径（不含 .tmp 后缀）。
func (mc *MusicCache) Store(cacheKey string, entry CacheEntry) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now().Format(time.RFC3339)
	entry.CachedAt = now
	entry.LastPlayed = now

	// 获取文件大小
	filePath := filepath.Join(mc.cacheDir, cacheKey+".mp3")
	if info, err := os.Stat(filePath); err == nil {
		entry.Size = info.Size()
	}

	mc.index[cacheKey] = &entry

	if err := mc.saveIndexLocked(); err != nil {
		return fmt.Errorf("保存缓存索引失败: %w", err)
	}

	// 检查并淘汰
	mc.evictLocked()

	logger.Infof("[cache] 已缓存: %s - %s (%s, %d bytes)", entry.Name, entry.Artist, cacheKey, entry.Size)
	return nil
}

// CacheDir 返回缓存目录路径。
func (mc *MusicCache) CacheDir() string {
	return mc.cacheDir
}

// List 返回所有缓存条目，按 last_played 倒序排列。
func (mc *MusicCache) List() []CacheEntry {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	results := make([]CacheEntry, 0, len(mc.index))
	for _, entry := range mc.index {
		results = append(results, *entry)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].LastPlayed > results[j].LastPlayed
	})

	return results
}

// Delete 删除指定缓存条目（按关键词匹配 name/artist，可选排除某些歌手）。
// 返回删除的条目数量。
func (mc *MusicCache) Delete(keyword string, excludeArtists []string) int {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	keyword = strings.ToLower(keyword)
	excludeSet := make(map[string]bool, len(excludeArtists))
	for _, a := range excludeArtists {
		excludeSet[strings.ToLower(a)] = true
	}

	deleted := 0
	for key, entry := range mc.index {
		nameLower := strings.ToLower(entry.Name)
		artistLower := strings.ToLower(entry.Artist)

		// 匹配关键词
		matched := strings.Contains(nameLower, keyword) || strings.Contains(artistLower, keyword) ||
			strings.Contains(keyword, nameLower) || strings.Contains(keyword, artistLower)
		if !matched {
			continue
		}

		// 检查是否在排除列表中
		if excludeSet[artistLower] {
			continue
		}

		// 删除文件和索引
		filePath := filepath.Join(mc.cacheDir, key+".mp3")
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			logger.Warnf("[cache] 删除缓存文件失败: %s: %v", filePath, err)
			continue
		}

		logger.Infof("[cache] 已删除缓存: %s - %s (%s)", entry.Name, entry.Artist, key)
		delete(mc.index, key)
		deleted++
	}

	if deleted > 0 {
		mc.saveIndexLocked()
	}

	return deleted
}

// DeleteByKey 删除指定 cacheKey 的缓存条目。
func (mc *MusicCache) DeleteByKey(cacheKey string) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, ok := mc.index[cacheKey]; !ok {
		return false
	}

	filePath := filepath.Join(mc.cacheDir, cacheKey+".mp3")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		logger.Warnf("[cache] 删除缓存文件失败: %s: %v", filePath, err)
		return false
	}

	delete(mc.index, cacheKey)
	mc.saveIndexLocked()
	return true
}

// FilePath 返回缓存文件的完整路径。
func (mc *MusicCache) FilePath(cacheKey string) string {
	return filepath.Join(mc.cacheDir, cacheKey+".mp3")
}

// TempFilePath 返回缓存临时文件的完整路径。
func (mc *MusicCache) TempFilePath(cacheKey string) string {
	return filepath.Join(mc.cacheDir, cacheKey+".mp3.tmp")
}

// loadIndex 从磁盘加载缓存索引。
func (mc *MusicCache) loadIndex() error {
	indexPath := filepath.Join(mc.cacheDir, "cache_index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &mc.index)
}

// saveIndexLocked 持久化缓存索引（调用方需持有锁）。
func (mc *MusicCache) saveIndexLocked() error {
	indexPath := filepath.Join(mc.cacheDir, "cache_index.json")
	data, err := json.MarshalIndent(mc.index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath, data, 0644)
}

// validateIndex 校验索引，移除本地文件不存在的条目。
func (mc *MusicCache) validateIndex() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	removed := 0
	for key := range mc.index {
		filePath := filepath.Join(mc.cacheDir, key+".mp3")
		if _, err := os.Stat(filePath); err != nil {
			delete(mc.index, key)
			removed++
		}
	}

	if removed > 0 {
		logger.Infof("[cache] 索引校验：移除 %d 个无效条目", removed)
		mc.saveIndexLocked()
	}

	logger.Infof("[cache] 缓存已加载: %d 首歌曲, 目录 %s", len(mc.index), mc.cacheDir)
}

// evictLocked 检查缓存总大小并淘汰最久未播放的（调用方需持有锁）。
func (mc *MusicCache) evictLocked() {
	if mc.maxSize <= 0 {
		return
	}

	// 计算总大小
	var totalSize int64
	for _, entry := range mc.index {
		totalSize += entry.Size
	}

	if totalSize <= mc.maxSize {
		return
	}

	// 按 last_played 升序排列，先淘汰最久未播放的
	type keyEntry struct {
		key   string
		entry *CacheEntry
	}
	var entries []keyEntry
	for k, v := range mc.index {
		entries = append(entries, keyEntry{key: k, entry: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].entry.LastPlayed < entries[j].entry.LastPlayed
	})

	for _, ke := range entries {
		if totalSize <= mc.maxSize {
			break
		}

		filePath := filepath.Join(mc.cacheDir, ke.key+".mp3")
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			logger.Warnf("[cache] 删除缓存文件失败: %s: %v", filePath, err)
			continue
		}

		totalSize -= ke.entry.Size
		delete(mc.index, ke.key)
		logger.Infof("[cache] LRU 淘汰: %s - %s (%s)", ke.entry.Name, ke.entry.Artist, ke.key)
	}

	mc.saveIndexLocked()
}
