package music

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
)

// cookieMaxAge 是 QQ 音乐 cookie 的最大有效期（经验值，通常 3 天左右会过期）。
const cookieMaxAge = 72 * time.Hour

// QQMusicClient QQ音乐客户端。
// 需要部署 QQMusicApi 服务：https://github.com/jsososo/QQMusicApi
type QQMusicClient struct {
	baseURL    string
	httpClient *http.Client
	dataDir    string

	cookieMu       sync.RWMutex
	cookies        []http.Cookie
	cookieTime     time.Time
	cookieFileTime time.Time // cookie 文件中的 updated_at
	cookieWarned   bool      // 是否已经发过过期警告（避免重复刷屏）
}

// NewQQMusicClient 创建 QQ 音乐客户端。
func NewQQMusicClient(baseURL string) *QQMusicClient {
	return NewQQMusicClientWithDataDir(baseURL, "")
}

// NewQQMusicClientWithDataDir 创建 QQ 音乐客户端（指定数据目录）。
func NewQQMusicClientWithDataDir(baseURL, dataDir string) *QQMusicClient {
	if dataDir == "" {
		dataDir = getDefaultDataDir()
	}
	return &QQMusicClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		dataDir: dataDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// loadCookies 加载 QQ 音乐 cookie（带缓存，每分钟最多读取一次文件）。
// 会检测 cookie 年龄，超过 cookieMaxAge 时打印警告日志。
func (c *QQMusicClient) loadCookies() []http.Cookie {
	c.cookieMu.RLock()
	if len(c.cookies) > 0 && time.Since(c.cookieTime) < time.Minute {
		cookies := c.cookies
		c.cookieMu.RUnlock()
		return cookies
	}
	c.cookieMu.RUnlock()

	c.cookieMu.Lock()
	defer c.cookieMu.Unlock()

	// 双重检查
	if len(c.cookies) > 0 && time.Since(c.cookieTime) < time.Minute {
		return c.cookies
	}

	path := filepath.Join(c.dataDir, "qq_cookie.json")
	content, err := os.ReadFile(path)
	if err != nil {
		if !c.cookieWarned {
			logger.Warnf("[qqmusic] 未找到 cookie 文件 %s，请先运行 pibuddy-music qq login 登录", path)
			c.cookieWarned = true
		}
		return nil
	}

	var data cookieFile
	if err := json.Unmarshal(content, &data); err != nil {
		logger.Warnf("[qqmusic] 解析 cookie 文件失败: %v", err)
		return nil
	}

	c.cookies = data.Cookies
	c.cookieTime = time.Now()
	c.cookieFileTime = data.UpdatedAt

	// 检测 cookie 年龄
	if !data.UpdatedAt.IsZero() && time.Since(data.UpdatedAt) > cookieMaxAge {
		age := time.Since(data.UpdatedAt).Round(time.Hour)
		if !c.cookieWarned {
			logger.Warnf("[qqmusic] QQ 音乐 cookie 已使用 %v，可能已过期，请运行 pibuddy-music qq login --web 重新登录", age)
			c.cookieWarned = true
		}
	} else {
		// cookie 被更新了（比如重新登录），重置警告标记
		c.cookieWarned = false
	}

	return c.cookies
}

// cookieHeader 生成 Cookie 请求头
func (c *QQMusicClient) cookieHeader() string {
	cookies := c.loadCookies()
	if len(cookies) == 0 {
		return ""
	}
	var parts []string
	for _, cookie := range cookies {
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(parts, "; ")
}

// doRequest 执行 HTTP 请求（自动附加 cookie）
func (c *QQMusicClient) doRequest(req *http.Request) (*http.Response, error) {
	if cookie := c.cookieHeader(); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	return c.httpClient.Do(req)
}

// cookieExpiredHint 返回 cookie 过期提示信息（附在错误末尾），如果 cookie 正常则返回空字符串。
func (c *QQMusicClient) cookieExpiredHint() string {
	c.cookieMu.RLock()
	defer c.cookieMu.RUnlock()

	if len(c.cookies) == 0 {
		return "（未登录，请运行 pibuddy-music qq login --web 登录）"
	}
	if !c.cookieFileTime.IsZero() && time.Since(c.cookieFileTime) > cookieMaxAge {
		return "（cookie 可能已过期，请运行 pibuddy-music qq login --web 重新登录）"
	}
	return ""
}

// qqSearchResult 搜索结果。
type qqSearchResult struct {
	Result int `json:"result"`
	Data   struct {
		List []struct {
			SongID      int    `json:"songid"`
			SongMID     string `json:"songmid"`
			SongName    string `json:"songname"`
			StrMediaMid string `json:"strMediaMid"`
			Singer      []struct {
				Name string `json:"name"`
			} `json:"singer"`
			AlbumName string `json:"albumname"`
		} `json:"list"`
	} `json:"data"`
}

// qqSongURLResult 歌曲 URL 结果。
type qqSongURLResult struct {
	Result int    `json:"result"`
	Data   string `json:"data"`
}

// qqSongDetail 歌曲详情（包含 mid）。
type qqSongDetail struct {
	ID  int64  `json:"id"`
	MID string `json:"mid"`
}

// Search 实现 Provider 接口：根据关键词搜索歌曲。
func (c *QQMusicClient) Search(ctx context.Context, keyword string, limit int) ([]Song, error) {
	// QQMusicApi 搜索接口
	apiURL := fmt.Sprintf("%s/search?key=%s&pageSize=%d", c.baseURL, url.QueryEscape(keyword), limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("请求 QQ 音乐 API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result qqSearchResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Result != 100 {
		hint := c.cookieExpiredHint()
		return nil, fmt.Errorf("QQ 音乐 API 返回错误: result=%d%s", result.Result, hint)
	}

	// 转换为统一格式
	var songs []Song
	for _, item := range result.Data.List {
		// 拼接歌手名
		var artists []string
		for _, s := range item.Singer {
			artists = append(artists, s.Name)
		}

		mediaMid := item.StrMediaMid
		if mediaMid == "" {
			mediaMid = item.SongMID
		}
		songs = append(songs, Song{
			ID:     int64(item.SongID),
			Name:   item.SongName,
			Artist: strings.Join(artists, "/"),
			Album:  item.AlbumName,
			// 存储 MID 和 MediaMID 用于获取 URL
			Extra: map[string]interface{}{
				"mid":       item.SongMID,
				"media_mid": mediaMid,
			},
		})
	}

	logger.Debugf("[qqmusic] 搜索 '%s' 返回 %d 首歌曲", keyword, len(songs))
	return songs, nil
}

// GetSongURL 实现 Provider 接口：获取歌曲播放地址。
func (c *QQMusicClient) GetSongURL(ctx context.Context, songID int64) (string, error) {
	// QQMusicApi 获取歌曲 URL 接口，id 参数传 songmid
	apiURL := fmt.Sprintf("%s/song/url?id=%d", c.baseURL, songID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("请求 QQ 音乐 API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var result qqSongURLResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Result != 100 {
		hint := c.cookieExpiredHint()
		return "", fmt.Errorf("QQ 音乐 API 返回错误: result=%d%s", result.Result, hint)
	}

	if result.Data == "" {
		return "", fmt.Errorf("无法获取歌曲播放地址，可能是 VIP 歌曲")
	}

	return result.Data, nil
}

// GetSongURLWithMID 使用 songMID 获取歌曲播放地址。
func (c *QQMusicClient) GetSongURLWithMID(ctx context.Context, songID int64, songMID string) (string, error) {
	// QQMusicApi /song/url 接口：id=songmid, mediaId=strMediaMid
	apiURL := fmt.Sprintf("%s/song/url?id=%s&mediaId=%s", c.baseURL, songMID, songMID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("请求 QQ 音乐 API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var result qqSongURLResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Result != 100 {
		hint := c.cookieExpiredHint()
		return "", fmt.Errorf("QQ 音乐 API 返回错误: result=%d%s", result.Result, hint)
	}

	if result.Data == "" {
		return "", fmt.Errorf("无法获取歌曲播放地址，可能是 VIP 歌曲")
	}

	return result.Data, nil
}

// parseSongID 解析歌曲 ID（支持字符串形式的 mid）。
func parseSongID(id int64) (int64, string) {
	return id, ""
}

// String 实现 Stringer 接口。
func (s Song) String() string {
	return fmt.Sprintf("%s - %s", s.Name, s.Artist)
}

// GetMID 从 Song 的 Extra 中获取 MID。
func (s Song) GetMID() string {
	if s.Extra == nil {
		return ""
	}
	if mid, ok := s.Extra["mid"]; ok {
		if midStr, ok := mid.(string); ok {
			return midStr
		}
	}
	return ""
}

// GetMediaMID 从 Song 的 Extra 中获取 MediaMID。
func (s Song) GetMediaMID() string {
	if s.Extra == nil {
		return ""
	}
	if mid, ok := s.Extra["media_mid"]; ok {
		if midStr, ok := mid.(string); ok {
			return midStr
		}
	}
	return s.GetMID() // fallback to songmid
}

// ParseSongID 从字符串解析歌曲 ID。
func ParseSongID(idStr string) int64 {
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}
