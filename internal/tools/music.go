package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iabetor/pibuddy/internal/music"
)

// MusicConfig 音乐服务配置。
type MusicConfig struct {
	Provider music.Provider
	History  *music.HistoryStore
	Enabled  bool
}

// MusicTool 播放音乐。
type MusicTool struct {
	provider music.Provider
	history  *music.HistoryStore
	enabled  bool
}

func NewMusicTool(cfg MusicConfig) *MusicTool {
	return &MusicTool{
		provider: cfg.Provider,
		history:  cfg.History,
		enabled:  cfg.Enabled,
	}
}

func (t *MusicTool) Name() string { return "play_music" }

func (t *MusicTool) Description() string {
	return "播放音乐。当用户想听歌、播放音乐时调用。"
}

func (t *MusicTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"keyword": {
				"type": "string",
				"description": "歌曲名或歌手名"
			}
		},
		"required": ["keyword"]
	}`)
}

// MusicResult 音乐播放结果，供 Pipeline 解析。
type MusicResult struct {
	Success   bool   `json:"success"`
	SongName  string `json:"song_name,omitempty"`
	Artist    string `json:"artist,omitempty"`
	URL       string `json:"url,omitempty"`
	Error     string `json:"error,omitempty"`
	NeedsVIP  bool   `json:"needs_vip,omitempty"`
}

func (t *MusicTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if !t.enabled || t.provider == nil {
		result := MusicResult{
			Success: false,
			Error:   "音乐服务未启用，请先部署 NeteaseCloudMusicApi",
		}
		return marshalResult(result)
	}

	var params struct {
		Keyword string `json:"keyword"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	if params.Keyword == "" {
		return "", fmt.Errorf("缺少 keyword 参数")
	}

	// 搜索歌曲
	songs, err := t.provider.Search(ctx, params.Keyword, 5)
	if err != nil {
		result := MusicResult{
			Success: false,
			Error:   fmt.Sprintf("搜索失败: %v", err),
		}
		return marshalResult(result)
	}

	if len(songs) == 0 {
		result := MusicResult{
			Success: false,
			Error:   "没有找到相关歌曲",
		}
		return marshalResult(result)
	}

	// 尝试获取播放 URL，优先选择可播放的歌曲
	for _, song := range songs {
		url, err := t.provider.GetSongURL(ctx, song.ID)
		if err != nil {
			continue // 跳过无法获取 URL 的歌曲
		}

		// 保存播放历史
		if t.history != nil {
			if err := t.history.Add(song); err != nil {
				// 仅记录日志，不影响播放
				fmt.Printf("[music] 保存播放历史失败: %v\n", err)
			}
		}

		result := MusicResult{
			Success:  true,
			SongName: song.Name,
			Artist:   song.Artist,
			URL:      url,
		}
		return marshalResult(result)
	}

	// 所有歌曲都无法获取 URL
	result := MusicResult{
		Success:  false,
		NeedsVIP: true,
		Error:    "找到的歌曲需要 VIP 会员才能播放",
	}
	return marshalResult(result)
}

func marshalResult(result MusicResult) (string, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("序列化结果失败: %w", err)
	}
	return string(data), nil
}

// ---- ListMusicHistoryTool 查看播放历史 ----

type ListMusicHistoryTool struct {
	history *music.HistoryStore
}

func NewListMusicHistoryTool(history *music.HistoryStore) *ListMusicHistoryTool {
	return &ListMusicHistoryTool{history: history}
}

func (t *ListMusicHistoryTool) Name() string { return "list_music_history" }

func (t *ListMusicHistoryTool) Description() string {
	return "查看播放历史。当用户说'播放历史'、'最近听了什么歌'等时使用。"
}

func (t *ListMusicHistoryTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"limit": {
				"type": "integer",
				"description": "返回的最大条数，默认10",
				"default": 10
			}
		},
		"required": []
	}`)
}

func (t *ListMusicHistoryTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if t.history == nil {
		return "播放历史功能未启用", nil
	}

	var params struct {
		Limit int `json:"limit"`
	}
	json.Unmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 10
	}

	entries := t.history.List(params.Limit)
	if len(entries) == 0 {
		return "还没有播放过任何歌曲", nil
	}

	result := fmt.Sprintf("最近播放的 %d 首歌:\n", len(entries))
	for i, e := range entries {
		result += fmt.Sprintf("%d. %s - %s (播放%d次, %s)\n", i+1, e.Name, e.Artist, e.PlayCount, e.PlayedAt)
	}
	return result, nil
}
