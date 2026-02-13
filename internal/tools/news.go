package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NewsTool 查询热点新闻。
type NewsTool struct {
	client *http.Client
}

func NewNewsTool() *NewsTool {
	return &NewsTool{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *NewsTool) Name() string { return "get_news" }

func (t *NewsTool) Description() string {
	return "获取当前热点新闻头条。当用户询问'今天有什么新闻'、'最近有什么大事'等时使用。"
}

func (t *NewsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"category": {
				"type": "string",
				"description": "新闻类别，可选值: 热榜、科技、娱乐、体育、财经。默认为热榜"
			}
		},
		"required": []
	}`)
}

// qqNewsResp 腾讯新闻热榜响应。
type qqNewsResp struct {
	Ret    int `json:"ret"`
	IDList []struct {
		NewsList []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			ArticleType string `json:"articletype"`
			HotEvent    struct {
				HotScore json.Number `json:"hotScore"`
				Ranking  int         `json:"ranking"`
			} `json:"hotEvent"`
		} `json:"newslist"`
	} `json:"idlist"`
}

func (t *NewsTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	// 使用腾讯新闻热榜 API（公开接口，无需注册）
	u := "https://r.inews.qq.com/gw/event/hot_ranking_list?page_size=15"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取新闻失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var qqResp qqNewsResp
	if err := json.Unmarshal(body, &qqResp); err != nil {
		return "", fmt.Errorf("解析新闻数据失败: %w", err)
	}

	if qqResp.Ret != 0 || len(qqResp.IDList) == 0 || len(qqResp.IDList[0].NewsList) == 0 {
		return "暂时无法获取新闻，请稍后再试。", nil
	}

	newsList := qqResp.IDList[0].NewsList

	// 取前 10 条有效新闻（跳过 articletype=560 的标题项）
	result := "今日热搜新闻:\n"
	count := 0
	for _, item := range newsList {
		if item.ArticleType == "560" || item.Title == "" {
			continue
		}
		count++
		result += fmt.Sprintf("%d. %s\n", count, item.Title)
		if count >= 10 {
			break
		}
	}

	if count == 0 {
		return "暂时无法获取新闻，请稍后再试。", nil
	}

	return result, nil
}
