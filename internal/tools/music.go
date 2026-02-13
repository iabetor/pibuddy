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
	return "搜索音乐。当用户想听歌时先调用此工具搜索，返回搜索结果列表，等待用户确认后再播放。"
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
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
}

func (t *SearchMusicTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if !t.enabled || t.provider == nil {
		result := SearchResult{
			Success: false,
			Error:   "音乐服务未启用，请先部署 NeteaseCloudMusicApi",
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
			ID:     s.ID,
			Name:   s.Name,
			Artist: s.Artist,
			Album:  s.Album,
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
	return "播放指定歌曲。用户确认后调用此工具播放。需要提供歌曲ID（从search_music结果中获取）。"
}

func (t *PlayMusicTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"song_id": {
				"type": "integer",
				"description": "歌曲ID，从search_music结果中获取"
			},
			"song_name": {
				"type": "string",
				"description": "歌曲名称（用于历史记录）"
			},
			"artist": {
				"type": "string",
				"description": "歌手名（用于历史记录）"
			}
		},
		"required": ["song_id", "song_name", "artist"]
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
			Error:   "音乐服务未启用，请先部署 NeteaseCloudMusicApi",
		}
		return marshalResult(result)
	}

	var params struct {
		SongID   int64  `json:"song_id"`
		SongName string `json:"song_name"`
		Artist   string `json:"artist"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	if params.SongID == 0 {
		return "", fmt.Errorf("缺少 song_id 参数")
	}

	// 获取播放 URL
	url, err := t.provider.GetSongURL(ctx, params.SongID)
	if err != nil {
		result := MusicResult{
			Success: false,
			Error:   fmt.Sprintf("获取播放地址失败: %v", err),
		}
		return marshalResult(result)
	}

	// 保存播放历史
	if t.history != nil {
		song := music.Song{
			ID:     params.SongID,
			Name:   params.SongName,
			Artist: params.Artist,
		}
		if err := t.history.Add(song); err != nil {
			fmt.Printf("[music] 保存播放历史失败: %v\n", err)
		}
	}

	result := MusicResult{
		Success:  true,
		SongName: params.SongName,
		Artist:   params.Artist,
		URL:      url,
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
