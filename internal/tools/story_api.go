package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
)

// StoryAPI 外部故事 API 客户端
type StoryAPI struct {
	baseURL    string
	httpClient *http.Client
	appID      string
	appSecret  string

	mu       sync.RWMutex
	cache    map[string]*cachedStory
	cacheTTL time.Duration
}

type cachedStory struct {
	story     *Story
	expiresAt time.Time
}

// NewStoryAPI 创建故事 API 客户端
func NewStoryAPI(baseURL, appID, appSecret string) *StoryAPI {
	return &StoryAPI{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		appID:      appID,
		appSecret:  appSecret,
		cache:      make(map[string]*cachedStory),
		cacheTTL:   7 * 24 * time.Hour, // 7 天缓存
	}
}

// mxnzpAPIResponse mxnzp API 响应格式
type mxnzpAPIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// mxnzpStoryType 故事分类
type mxnzpStoryType struct {
	Name   string `json:"name"`
	TypeID int    `json:"type_id"`
}

// mxnzpStoryItem 故事条目（列表/搜索接口返回）
type mxnzpStoryItem struct {
	ID       int    `json:"storyId"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	Length   int    `json:"length"`
	ReadTime string `json:"readTime"`
}

// mxnzpStoryDetail 故事详情
type mxnzpStoryDetail struct {
	ID       int    `json:"storyId"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Type     string `json:"type"`
	Length   int    `json:"length"`
	ReadTime string `json:"readTime"`
}

// GetTypes 获取故事分类
func (a *StoryAPI) GetTypes(ctx context.Context) ([]mxnzpStoryType, error) {
	logger.Debugf("[story] GetTypes app_id=%s", a.appID)
	apiURL := fmt.Sprintf("%s/api/story/types?app_id=%s&app_secret=%s", a.baseURL, a.appID, a.appSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data []mxnzpStoryType `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != 1 {
		return nil, fmt.Errorf("API error: %s", result.Msg)
	}

	return result.Data, nil
}

// GetList 获取故事列表（支持在当前分类下按关键字模糊搜索）
func (a *StoryAPI) GetList(ctx context.Context, typeID int, keyword string, page int) ([]mxnzpStoryItem, error) {
	apiURL := fmt.Sprintf("%s/api/story/list?type_id=%d&page=%d&app_id=%s&app_secret=%s",
		a.baseURL, typeID, page, a.appID, a.appSecret)
	if keyword != "" {
		apiURL += "&keyword=" + url.QueryEscape(keyword)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code int              `json:"code"`
		Msg  string           `json:"msg"`
		Data []mxnzpStoryItem `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != 1 {
		return nil, fmt.Errorf("API error: %s", result.Msg)
	}

	return result.Data, nil
}

// GetContent 获取故事详情
func (a *StoryAPI) GetContent(ctx context.Context, id int) (*Story, error) {
	cacheKey := fmt.Sprintf("api_%d", id)

	// 检查缓存
	a.mu.RLock()
	if cached, ok := a.cache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		a.mu.RUnlock()
		return cached.story, nil
	}
	a.mu.RUnlock()

	// 使用正确的接口：/api/story/details?story_id=xxx
	apiURL := fmt.Sprintf("%s/api/story/details?story_id=%d&app_id=%s&app_secret=%s", a.baseURL, id, a.appID, a.appSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code int               `json:"code"`
		Msg  string            `json:"msg"`
		Data mxnzpStoryDetail `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != 1 {
		return nil, fmt.Errorf("API error: %s", result.Msg)
	}

	story := &Story{
		ID:       fmt.Sprintf("api_%d", result.Data.ID),
		Title:    result.Data.Title,
		Category: result.Data.Type,
		Content:  result.Data.Content,
		Source:   "api",
	}

	// 缓存结果
	a.mu.Lock()
	a.cache[cacheKey] = &cachedStory{
		story:     story,
		expiresAt: time.Now().Add(a.cacheTTL),
	}
	a.mu.Unlock()

	return story, nil
}

// FindStory 通过 API 搜索故事
func (a *StoryAPI) FindStory(ctx context.Context, keyword string) (*Story, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, nil
	}

	// 检查缓存
	cacheKey := "search_" + keyword
	a.mu.RLock()
	if cached, ok := a.cache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		a.mu.RUnlock()
		return cached.story, nil
	}
	a.mu.RUnlock()

	// 使用搜索 API
	apiURL := fmt.Sprintf("%s/api/story/search?keyword=%s&page=1&app_id=%s&app_secret=%s",
		a.baseURL, url.QueryEscape(keyword), a.appID, a.appSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code int              `json:"code"`
		Msg  string           `json:"msg"`
		Data []mxnzpStoryItem `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != 1 {
		logger.Debugf("[story] 搜索失败: %s", result.Msg)
		return nil, nil
	}

	logger.Debugf("[story] 搜索到 %d 个结果", len(result.Data))

	// 取第一个匹配结果
	if len(result.Data) > 0 {
		item := result.Data[0]
		logger.Debugf("[story] 匹配成功: %s", item.Title)

		// 等待 1.1 秒避免 QPS 限制（普通会员 QPS=1）
		time.Sleep(1100 * time.Millisecond)

		return a.GetContent(ctx, item.ID)
	}

	return nil, nil
}

// SearchStory 搜索故事（带分页，基于所有分类进行全局搜索）
func (a *StoryAPI) SearchStory(ctx context.Context, keyword string, page int) ([]mxnzpStoryItem, error) {
	apiURL := fmt.Sprintf("%s/api/story/search?keyword=%s&page=%d&app_id=%s&app_secret=%s",
		a.baseURL, url.QueryEscape(keyword), page, a.appID, a.appSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code int              `json:"code"`
		Msg  string           `json:"msg"`
		Data []mxnzpStoryItem `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != 1 {
		return nil, fmt.Errorf("API error: %s", result.Msg)
	}

	return result.Data, nil
}
