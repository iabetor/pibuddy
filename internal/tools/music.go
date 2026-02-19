package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iabetor/pibuddy/internal/logger"
	"github.com/iabetor/pibuddy/internal/music"
)

// MusicConfig 音乐服务配置。
type MusicConfig struct {
	Provider music.Provider
	History  *music.HistoryStore
	Enabled  bool
}

// ---- SearchMusicTool 搜索音乐 ----

type SearchMusicTool struct {
	provider music.Provider
	enabled  bool
}

func NewSearchMusicTool(cfg MusicConfig) *SearchMusicTool {
	return &SearchMusicTool{
		provider: cfg.Provider,
		enabled:  cfg.Enabled,
	}
}

func (t *SearchMusicTool) Name() string { return "search_music" }

func (t *SearchMusicTool) Description() string {
	return "搜索音乐。仅在用户明确要求'搜索'、'查找'歌曲而非播放时使用。如果用户想听歌，请直接使用 play_music 工具。"
}

func (t *SearchMusicTool) Parameters() json.RawMessage {
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

// SearchResult 搜索结果，供 LLM 展示给用户。
type SearchResult struct {
	Success bool    `json:"success"`
	Songs   []SongInfo `json:"songs,omitempty"`
	Error   string  `json:"error,omitempty"`
}

type SongInfo struct {
	ID       int64  `json:"id"`
	MID      string `json:"mid,omitempty"`       // QQ 音乐 songmid
	MediaMID string `json:"media_mid,omitempty"` // QQ 音乐 strMediaMid
	Name     string `json:"name"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
}

func (t *SearchMusicTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if !t.enabled || t.provider == nil {
		result := SearchResult{
			Success: false,
			Error:   "音乐服务未启用，请先部署音乐 API 服务",
		}
		return marshalMusicResult(result)
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
		result := SearchResult{
			Success: false,
			Error:   fmt.Sprintf("搜索失败: %v", err),
		}
		return marshalMusicResult(result)
	}

	if len(songs) == 0 {
		result := SearchResult{
			Success: false,
			Error:   "没有找到相关歌曲",
		}
		return marshalMusicResult(result)
	}

	// 返回搜索结果列表
	songInfos := make([]SongInfo, len(songs))
	for i, s := range songs {
		songInfos[i] = SongInfo{
			ID:       s.ID,
			MID:      s.GetMID(),
			MediaMID: s.GetMediaMID(),
			Name:     s.Name,
			Artist:   s.Artist,
			Album:    s.Album,
		}
	}

	result := SearchResult{
		Success: true,
		Songs:   songInfos,
	}
	return marshalMusicResult(result)
}

// ---- PlayMusicTool 播放指定音乐 ----

type PlayMusicTool struct {
	provider music.Provider
	history  *music.HistoryStore
	enabled  bool
}

func NewPlayMusicTool(cfg MusicConfig) *PlayMusicTool {
	return &PlayMusicTool{
		provider: cfg.Provider,
		history:  cfg.History,
		enabled:  cfg.Enabled,
	}
}

func (t *PlayMusicTool) Name() string { return "play_music" }

func (t *PlayMusicTool) Description() string {
	return "播放音乐。当用户想听歌时直接调用此工具，只需提供关键词（歌名、歌手名等），会自动搜索并播放最匹配的歌曲。如果第一首因版权限制无法播放，会自动尝试下一首。"
}

func (t *PlayMusicTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"keyword": {
				"type": "string",
				"description": "歌曲名、歌手名或其组合，例如'周杰伦晴天'"
			}
		},
		"required": ["keyword"]
	}`)
}

// MusicResult 音乐播放结果，供 Pipeline 解析。
type MusicResult struct {
	Success  bool   `json:"success"`
	SongName string `json:"song_name,omitempty"`
	Artist   string `json:"artist,omitempty"`
	URL      string `json:"url,omitempty"`
	Error    string `json:"error,omitempty"`
	NeedsVIP bool   `json:"needs_vip,omitempty"`
}

func (t *PlayMusicTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if !t.enabled || t.provider == nil {
		result := MusicResult{
			Success: false,
			Error:   "音乐服务未启用，请先部署音乐 API 服务",
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

	// 搜索歌曲（多取几首用于 fallback）
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

	// 依次尝试获取播放 URL，跳过无版权 / VIP 歌曲
	qqProvider, isQQ := t.provider.(music.QQProvider)

	for i, song := range songs {
		var songURL string
		var urlErr error

		if isQQ {
			mid := song.GetMID()
			if mid != "" {
				songURL, urlErr = qqProvider.GetSongURLWithMID(ctx, song.ID, mid)
			} else {
				songURL, urlErr = t.provider.GetSongURL(ctx, song.ID)
			}
		} else {
			songURL, urlErr = t.provider.GetSongURL(ctx, song.ID)
		}

		if urlErr != nil {
			logger.Debugf("[music] 第 %d 首 %s - %s 无法播放: %v，尝试下一首", i+1, song.Name, song.Artist, urlErr)
			continue
		}

		if songURL == "" {
			logger.Debugf("[music] 第 %d 首 %s - %s URL 为空，尝试下一首", i+1, song.Name, song.Artist)
			continue
		}

		// 找到可播放的歌曲
		if t.history != nil {
			if addErr := t.history.Add(song); addErr != nil {
				logger.Debugf("[music] 保存播放历史失败: %v", addErr)
			}
		}

		result := MusicResult{
			Success:  true,
			SongName: song.Name,
			Artist:   song.Artist,
			URL:      songURL,
		}
		if i > 0 {
			logger.Infof("[music] 前 %d 首无法播放，已自动切换到: %s - %s", i, song.Name, song.Artist)
		}
		return marshalResult(result)
	}

	// 所有候选歌曲都无法播放
	result := MusicResult{
		Success: false,
		Error:   fmt.Sprintf("搜索到 %d 首歌曲，但均因版权限制无法播放", len(songs)),
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

func marshalMusicResult(result SearchResult) (string, error) {
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
