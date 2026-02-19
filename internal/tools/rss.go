package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/iabetor/pibuddy/internal/rss"
)

// ---- AddRSSFeedTool ----

// AddRSSFeedTool 添加 RSS 订阅源。
type AddRSSFeedTool struct {
	store   *rss.FeedStore
	fetcher *rss.Fetcher
}

// NewAddRSSFeedTool 创建添加订阅源工具。
func NewAddRSSFeedTool(store *rss.FeedStore, fetcher *rss.Fetcher) *AddRSSFeedTool {
	return &AddRSSFeedTool{store: store, fetcher: fetcher}
}

func (t *AddRSSFeedTool) Name() string { return "add_rss_feed" }
func (t *AddRSSFeedTool) Description() string {
	return "添加 RSS 订阅源。当用户说'订阅某某网站'、'帮我添加RSS'等时使用。需要提供 RSS/Atom 源的 URL 地址。"
}
func (t *AddRSSFeedTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "RSS/Atom 订阅源 URL"
			},
			"name": {
				"type": "string",
				"description": "订阅源名称（可选，不提供则自动从 Feed 标题获取）"
			}
		},
		"required": ["url"]
	}`)
}

func (t *AddRSSFeedTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	if params.URL == "" {
		return "", fmt.Errorf("缺少 url 参数")
	}

	// 验证 URL 有效性并获取标题
	title, err := t.fetcher.FetchAndValidate(ctx, params.URL)
	if err != nil {
		return fmt.Sprintf("无法解析该 RSS 地址，请检查 URL 是否正确: %v", err), nil
	}

	name := params.Name
	if name == "" {
		name = title
	}

	feed := rss.Feed{
		Name: name,
		URL:  params.URL,
	}
	if err := t.store.Add(feed); err != nil {
		return err.Error(), nil
	}

	return fmt.Sprintf("已成功订阅 %s", name), nil
}

// ---- ListRSSFeedsTool ----

// ListRSSFeedsTool 列出所有 RSS 订阅源。
type ListRSSFeedsTool struct {
	store *rss.FeedStore
}

// NewListRSSFeedsTool 创建列出订阅源工具。
func NewListRSSFeedsTool(store *rss.FeedStore) *ListRSSFeedsTool {
	return &ListRSSFeedsTool{store: store}
}

func (t *ListRSSFeedsTool) Name() string { return "list_rss_feeds" }
func (t *ListRSSFeedsTool) Description() string {
	return "查看所有 RSS 订阅源。当用户说'我订阅了哪些'、'有哪些RSS'等时使用。"
}
func (t *ListRSSFeedsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}

func (t *ListRSSFeedsTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	feeds := t.store.List()
	if len(feeds) == 0 {
		return "当前没有任何 RSS 订阅。可以告诉我想订阅的网站 RSS 地址来添加。", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("当前有 %d 个订阅源:\n", len(feeds)))
	for i, f := range feeds {
		sb.WriteString(fmt.Sprintf("%d. %s (%s)", i+1, f.Name, f.URL))
		if !f.LastFetched.IsZero() {
			sb.WriteString(fmt.Sprintf(" [上次更新: %s]", f.LastFetched.Format("01-02 15:04")))
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// ---- DeleteRSSFeedTool ----

// DeleteRSSFeedTool 删除 RSS 订阅源。
type DeleteRSSFeedTool struct {
	store *rss.FeedStore
}

// NewDeleteRSSFeedTool 创建删除订阅源工具。
func NewDeleteRSSFeedTool(store *rss.FeedStore) *DeleteRSSFeedTool {
	return &DeleteRSSFeedTool{store: store}
}

func (t *DeleteRSSFeedTool) Name() string { return "delete_rss_feed" }
func (t *DeleteRSSFeedTool) Description() string {
	return "删除 RSS 订阅源。当用户说'取消订阅某某'、'删除RSS'等时使用。"
}
func (t *DeleteRSSFeedTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {
				"type": "string",
				"description": "订阅源 ID 或名称"
			}
		},
		"required": ["id"]
	}`)
}

func (t *DeleteRSSFeedTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	if params.ID == "" {
		return "", fmt.Errorf("缺少 id 参数")
	}

	if t.store.Delete(params.ID) {
		return fmt.Sprintf("已取消订阅 %s", params.ID), nil
	}
	return fmt.Sprintf("未找到订阅源 %s", params.ID), nil
}

// ---- GetRSSNewsTool ----

// GetRSSNewsTool 获取 RSS 最新内容。
type GetRSSNewsTool struct {
	store   *rss.FeedStore
	fetcher *rss.Fetcher
}

// NewGetRSSNewsTool 创建获取 RSS 内容工具。
func NewGetRSSNewsTool(store *rss.FeedStore, fetcher *rss.Fetcher) *GetRSSNewsTool {
	return &GetRSSNewsTool{store: store, fetcher: fetcher}
}

func (t *GetRSSNewsTool) Name() string { return "get_rss_news" }
func (t *GetRSSNewsTool) Description() string {
	return "获取 RSS 订阅源的最新内容。当用户说'有什么新消息'、'看看RSS'、'读读订阅'等时使用。支持按来源和关键词过滤。"
}
func (t *GetRSSNewsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"source": {
				"type": "string",
				"description": "订阅源名称，用于过滤指定来源的内容（可选）"
			},
			"keyword": {
				"type": "string",
				"description": "关键词，用于过滤标题或摘要中包含该词的内容（可选）"
			},
			"limit": {
				"type": "integer",
				"description": "返回条目数量，默认5条"
			}
		},
		"required": []
	}`)
}

func (t *GetRSSNewsTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Source  string `json:"source"`
		Keyword string `json:"keyword"`
		Limit   int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	feeds := t.store.List()
	if len(feeds) == 0 {
		return "你还没有添加任何 RSS 订阅，可以告诉我想订阅的网站地址。", nil
	}

	items, err := t.fetcher.GetNews(ctx, params.Source, params.Keyword, params.Limit)
	if err != nil {
		return fmt.Sprintf("获取内容失败: %v", err), nil
	}

	if len(items) == 0 {
		msg := "没有找到相关内容"
		if params.Keyword != "" {
			msg += fmt.Sprintf("（关键词: %s）", params.Keyword)
		}
		return msg, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("最新 %d 条内容:\n", len(items)))
	for i, item := range items {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, item.FeedName, item.Title))
		if !item.Published.IsZero() {
			sb.WriteString(fmt.Sprintf(" (%s)", item.Published.Format(time.DateOnly)))
		}
		sb.WriteString("\n")
		if item.Summary != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", item.Summary))
		}
	}
	return sb.String(), nil
}
