package rss

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/iabetor/pibuddy/internal/logger"
	"github.com/mmcdole/gofeed"
)

const (
	defaultCacheTTL     = 30 * time.Minute
	defaultMaxItems     = 20 // 每个 Feed 缓存的最大条目数
	defaultFetchTimeout = 10 * time.Second
	maxSummaryLen       = 200 // 摘要最大字符数
)

// Fetcher 负责抓取和缓存 RSS 内容。
type Fetcher struct {
	mu        sync.RWMutex
	store     *FeedStore
	cachePath string
	cache     map[string]cachedFeed // key: feed ID
	cacheTTL  time.Duration
	parser    *gofeed.Parser
	client    *http.Client
}

// cachedFeed 单个 Feed 的缓存。
type cachedFeed struct {
	FetchedAt time.Time  `json:"fetched_at"`
	Items     []FeedItem `json:"items"`
}

// NewFetcher 创建 RSS 内容抓取器。
func NewFetcher(store *FeedStore, dataDir string, cacheTTLMinutes int) *Fetcher {
	ttl := defaultCacheTTL
	if cacheTTLMinutes > 0 {
		ttl = time.Duration(cacheTTLMinutes) * time.Minute
	}

	f := &Fetcher{
		store:     store,
		cachePath: filepath.Join(dataDir, "rss_cache.json"),
		cache:     make(map[string]cachedFeed),
		cacheTTL:  ttl,
		parser:    gofeed.NewParser(),
		client:    &http.Client{Timeout: defaultFetchTimeout},
	}

	// 加载缓存
	if err := f.loadCache(); err != nil {
		logger.Debugf("[rss] 加载缓存失败: %v", err)
	}

	return f
}

// FetchAndValidate 抓取指定 URL 的 Feed，验证有效性并返回 Feed 标题。
func (f *Fetcher) FetchAndValidate(ctx context.Context, url string) (string, error) {
	feed, err := f.parseFeed(ctx, url)
	if err != nil {
		return "", fmt.Errorf("无法解析该 RSS 地址: %w", err)
	}
	title := feed.Title
	if title == "" {
		title = url
	}
	return title, nil
}

// GetNews 获取订阅源的最新内容。
// source 为空则获取所有源，keyword 为空则不过滤。
func (f *Fetcher) GetNews(ctx context.Context, source string, keyword string, limit int) ([]FeedItem, error) {
	if limit <= 0 {
		limit = 5
	}

	feeds := f.store.List()
	if len(feeds) == 0 {
		return nil, nil
	}

	// 过滤源
	if source != "" {
		var filtered []Feed
		lower := strings.ToLower(source)
		for _, fd := range feeds {
			if strings.Contains(strings.ToLower(fd.Name), lower) {
				filtered = append(filtered, fd)
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("未找到名为 %q 的订阅源", source)
		}
		feeds = filtered
	}

	var allItems []FeedItem
	for _, fd := range feeds {
		items, err := f.getFeedItems(ctx, fd)
		if err != nil {
			logger.Warnf("[rss] 获取 %s 失败: %v", fd.Name, err)
			continue
		}
		allItems = append(allItems, items...)
	}

	// 按发布时间倒序
	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].Published.After(allItems[j].Published)
	})

	// 按关键词过滤
	if keyword != "" {
		lower := strings.ToLower(keyword)
		var filtered []FeedItem
		for _, item := range allItems {
			if strings.Contains(strings.ToLower(item.Title), lower) ||
				strings.Contains(strings.ToLower(item.Summary), lower) {
				filtered = append(filtered, item)
			}
		}
		allItems = filtered
	}

	if len(allItems) > limit {
		allItems = allItems[:limit]
	}
	return allItems, nil
}

// getFeedItems 获取单个 Feed 的条目（优先使用缓存）。
func (f *Fetcher) getFeedItems(ctx context.Context, fd Feed) ([]FeedItem, error) {
	f.mu.RLock()
	cached, hasCached := f.cache[fd.ID]
	f.mu.RUnlock()

	if hasCached && time.Since(cached.FetchedAt) < f.cacheTTL {
		return cached.Items, nil
	}

	// 缓存过期或不存在，重新抓取
	feed, err := f.parseFeed(ctx, fd.URL)
	if err != nil {
		// 抓取失败但有旧缓存，使用旧缓存
		if hasCached {
			logger.Warnf("[rss] 抓取 %s 失败，使用旧缓存: %v", fd.Name, err)
			return cached.Items, nil
		}
		return nil, err
	}

	items := f.convertItems(feed, fd.Name)

	// 更新缓存
	f.mu.Lock()
	f.cache[fd.ID] = cachedFeed{
		FetchedAt: time.Now(),
		Items:     items,
	}
	f.mu.Unlock()

	// 异步保存缓存和更新抓取时间
	go func() {
		f.saveCache()
		f.store.UpdateLastFetched(fd.ID, time.Now())
	}()

	return items, nil
}

// parseFeed 解析 Feed URL。
func (f *Fetcher) parseFeed(ctx context.Context, url string) (*gofeed.Feed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "PiBuddy/1.0 RSS Reader")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return f.parser.Parse(resp.Body)
}

// convertItems 将 gofeed 条目转换为 FeedItem。
func (f *Fetcher) convertItems(feed *gofeed.Feed, feedName string) []FeedItem {
	maxItems := defaultMaxItems
	if len(feed.Items) < maxItems {
		maxItems = len(feed.Items)
	}

	items := make([]FeedItem, 0, maxItems)
	for i := 0; i < maxItems; i++ {
		gItem := feed.Items[i]

		summary := gItem.Description
		if summary == "" {
			summary = gItem.Content
		}
		summary = stripHTML(summary)
		summary = truncate(summary, maxSummaryLen)

		published := time.Now()
		if gItem.PublishedParsed != nil {
			published = *gItem.PublishedParsed
		} else if gItem.UpdatedParsed != nil {
			published = *gItem.UpdatedParsed
		}

		items = append(items, FeedItem{
			Title:     gItem.Title,
			Summary:   summary,
			Link:      gItem.Link,
			Published: published,
			FeedName:  feedName,
		})
	}
	return items
}

func (f *Fetcher) loadCache() error {
	data, err := os.ReadFile(f.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &f.cache)
}

func (f *Fetcher) saveCache() {
	f.mu.RLock()
	defer f.mu.RUnlock()

	data, err := json.Marshal(f.cache)
	if err != nil {
		logger.Debugf("[rss] 序列化缓存失败: %v", err)
		return
	}
	if err := os.WriteFile(f.cachePath, data, 0644); err != nil {
		logger.Debugf("[rss] 保存缓存失败: %v", err)
	}
}

// stripHTML 剥离 HTML 标签，只保留纯文本。
func stripHTML(s string) string {
	// 移除 HTML 标签
	re := regexp.MustCompile(`<[^>]*>`)
	s = re.ReplaceAllString(s, "")

	// 处理常见 HTML 实体
	replacer := strings.NewReplacer(
		"&nbsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"&apos;", "'",
	)
	s = replacer.Replace(s)

	// 合并连续空白
	spaceRe := regexp.MustCompile(`\s+`)
	s = spaceRe.ReplaceAllString(s, " ")

	return strings.TrimSpace(s)
}

// truncate 截断字符串到指定字符数（按 UTF-8 字符计算）。
func truncate(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen]) + "..."
}
