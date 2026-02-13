package music

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// NeteaseClient 是网易云音乐 API 客户端。
type NeteaseClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewNeteaseClient 创建网易云音乐客户端。
func NewNeteaseClient(baseURL string) *NeteaseClient {
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}
	return &NeteaseClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// searchResponse 搜索 API 响应结构。
type searchResponse struct {
	Code int `json:"code"`
	Result struct {
		Songs []struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
			Album struct {
				Name string `json:"name"`
			} `json:"album"`
		} `json:"songs"`
	} `json:"result"`
}

// songURLResponse 获取歌曲 URL 响应结构。
type songURLResponse struct {
	Code int `json:"code"`
	Data []struct {
		URL           string `json:"url"`
		FreeTrialInfo *struct {
			Start int `json:"start"`
			End   int `json:"end"`
		} `json:"freeTrialInfo"`
	} `json:"data"`
}

// Search 根据关键词搜索歌曲。
func (c *NeteaseClient) Search(ctx context.Context, keyword string, limit int) ([]Song, error) {
	if limit <= 0 {
		limit = 10
	}

	// 构建请求 URL
	u := fmt.Sprintf("%s/search?keywords=%s&limit=%d", c.baseURL, url.QueryEscape(keyword), limit)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("搜索请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("搜索请求返回错误状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var searchResp searchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if searchResp.Code != 200 {
		return nil, fmt.Errorf("搜索失败，错误码: %d", searchResp.Code)
	}

	// 转换为 Song 结构
	songs := make([]Song, 0, len(searchResp.Result.Songs))
	for _, s := range searchResp.Result.Songs {
		artist := ""
		if len(s.Artists) > 0 {
			artist = s.Artists[0].Name
		}
		songs = append(songs, Song{
			ID:     s.ID,
			Name:   s.Name,
			Artist: artist,
			Album:  s.Album.Name,
		})
	}

	return songs, nil
}

// GetSongURL 获取歌曲播放地址。
// 返回 URL 和是否为试听版。
func (c *NeteaseClient) GetSongURL(ctx context.Context, songID int64) (string, error) {
	// 尝试获取最高音质，避免试听版
	u := fmt.Sprintf("%s/song/url?id=%d&br=999000", c.baseURL, songID)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取播放地址请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("获取播放地址返回错误状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var urlResp songURLResponse
	if err := json.Unmarshal(body, &urlResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if urlResp.Code != 200 {
		return "", fmt.Errorf("获取播放地址失败，错误码: %d", urlResp.Code)
	}

	if len(urlResp.Data) == 0 || urlResp.Data[0].URL == "" {
		return "", fmt.Errorf("无法获取播放地址，可能需要 VIP")
	}

	// 检查是否为试听版
	if urlResp.Data[0].FreeTrialInfo != nil {
		return "", fmt.Errorf("该歌曲需要 VIP 会员")
	}

	return urlResp.Data[0].URL, nil
}
