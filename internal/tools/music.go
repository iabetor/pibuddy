package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iabetor/pibuddy/internal/audio"
	"github.com/iabetor/pibuddy/internal/logger"
	"github.com/iabetor/pibuddy/internal/music"
)

// MusicConfig 音乐服务配置。
type MusicConfig struct {
	Provider music.Provider
	History  *music.HistoryStore
	Playlist *music.Playlist
	Cache    *audio.MusicCache
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
	playlist *music.Playlist
	cache    *audio.MusicCache
	enabled  bool
}

func NewPlayMusicTool(cfg MusicConfig) *PlayMusicTool {
	return &PlayMusicTool{
		provider: cfg.Provider,
		history:  cfg.History,
		playlist: cfg.Playlist,
		cache:    cfg.Cache,
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
	Success      bool    `json:"success"`
	SongName     string  `json:"song_name,omitempty"`
	Artist       string  `json:"artist,omitempty"`
	URL          string  `json:"url,omitempty"`
	CacheKey     string  `json:"cache_key,omitempty"`    // 缓存标识，如 "qq_12345678"
	Error        string  `json:"error,omitempty"`
	NeedsVIP     bool    `json:"needs_vip,omitempty"`
	PlaylistSize int     `json:"playlist_size,omitempty"` // 播放列表中的总歌曲数
	Message      string  `json:"message,omitempty"`       // 附加消息（如恢复播放信息）
	PositionSec  float64 `json:"position_sec,omitempty"`  // 从指定位置开始播放（秒）
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

	// 1. 先查本地缓存（离线优先）
	if t.cache != nil && t.cache.Enabled() {
		cachedItems := t.cache.Search(params.Keyword)
		if len(cachedItems) > 0 {
			logger.Infof("[music] 缓存命中 %d 首: %s", len(cachedItems), params.Keyword)

			var playlistItems []music.PlaylistItem
			for _, ci := range cachedItems {
				cacheKey := fmt.Sprintf("%s_%d", ci.Provider, ci.ID)
				playlistItems = append(playlistItems, music.PlaylistItem{
					Song: music.Song{
						ID:     ci.ID,
						Name:   ci.Name,
						Artist: ci.Artist,
						Album:  ci.Album,
					},
					URL:      "", // 从缓存播放不需要 URL
					CacheKey: cacheKey,
				})
			}

			firstItem := playlistItems[0]
			firstCacheKey := firstItem.CacheKey

			if t.playlist != nil && len(playlistItems) > 0 {
				t.playlist.Replace(playlistItems)
				t.playlist.Next(ctx)
			}

			if t.history != nil {
				if addErr := t.history.Add(firstItem.Song); addErr != nil {
					logger.Debugf("[music] 保存播放历史失败: %v", addErr)
				}
			}

			result := MusicResult{
				Success:      true,
				SongName:     firstItem.Song.Name,
				Artist:       firstItem.Song.Artist,
				CacheKey:     firstCacheKey,
				PlaylistSize: len(playlistItems),
			}
			return marshalResult(result)
		}
	}

	// 2. 缓存未命中，走原有的网络搜索流程
	songs, err := t.provider.Search(ctx, params.Keyword, 10)
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

	providerName := t.provider.ProviderName()

	// 依次尝试获取播放 URL，跳过无版权 / VIP 歌曲
	qqProvider, isQQ := t.provider.(music.QQProvider)

	var firstURL string
	var firstSong music.Song
	var firstCacheKey string
	var playlistItems []music.PlaylistItem

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
			logger.Debugf("[music] 第 %d 首 %s - %s 无法播放: %v，跳过", i+1, song.Name, song.Artist, urlErr)
			continue
		}

		if songURL == "" {
			logger.Debugf("[music] 第 %d 首 %s - %s URL 为空，跳过", i+1, song.Name, song.Artist)
			continue
		}

		cacheKey := fmt.Sprintf("%s_%d", providerName, song.ID)

		playlistItems = append(playlistItems, music.PlaylistItem{
			Song:     song,
			URL:      songURL,
			CacheKey: cacheKey,
		})

		if firstURL == "" {
			firstURL = songURL
			firstSong = song
			firstCacheKey = cacheKey
		}
	}

	if firstURL == "" {
		result := MusicResult{
			Success: false,
			Error:   fmt.Sprintf("搜索到 %d 首歌曲，但均因版权限制无法播放", len(songs)),
		}
		return marshalResult(result)
	}

	// 将所有可播放歌曲放入播放列表
	if t.playlist != nil && len(playlistItems) > 0 {
		t.playlist.Replace(playlistItems)
		t.playlist.Next(ctx)
		logger.Infof("[music] 已将 %d 首歌曲加入播放列表", len(playlistItems))
	}

	// 记录播放历史
	if t.history != nil {
		if addErr := t.history.Add(firstSong); addErr != nil {
			logger.Debugf("[music] 保存播放历史失败: %v", addErr)
		}
	}

	result := MusicResult{
		Success:      true,
		SongName:     firstSong.Name,
		Artist:       firstSong.Artist,
		URL:          firstURL,
		CacheKey:     firstCacheKey,
		PlaylistSize: len(playlistItems),
	}
	if len(playlistItems) > 1 {
		logger.Infof("[music] 第一首: %s - %s，列表共 %d 首", firstSong.Name, firstSong.Artist, len(playlistItems))
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

// ---- NextMusicTool 切换下一首 ----

// NextMusicTool 切换到播放列表中的下一首歌曲。
type NextMusicTool struct {
	playlist *music.Playlist
}

func NewNextMusicTool(playlist *music.Playlist) *NextMusicTool {
	return &NextMusicTool{playlist: playlist}
}

func (t *NextMusicTool) Name() string { return "next_music" }

func (t *NextMusicTool) Description() string {
	return "切换到下一首歌。当用户说'下一首'、'换一首'、'跳过'等时使用。"
}

func (t *NextMusicTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`)
}

func (t *NextMusicTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if t.playlist == nil || t.playlist.Len() == 0 {
		result := MusicResult{
			Success: false,
			Error:   "当前没有播放列表",
		}
		return marshalResult(result)
	}

	url, songName, artist, cacheKey, ok := t.playlist.Next(ctx)
	if !ok {
		result := MusicResult{
			Success: false,
			Error:   "播放列表已到末尾，没有更多歌曲了",
		}
		return marshalResult(result)
	}

	result := MusicResult{
		Success:      true,
		SongName:     songName,
		Artist:       artist,
		URL:          url,
		CacheKey:     cacheKey,
		PlaylistSize: t.playlist.Len(),
	}
	return marshalResult(result)
}

// ---- SetPlayModeTool 设置播放模式 ----

type SetPlayModeTool struct {
	playlist *music.Playlist
}

func NewSetPlayModeTool(playlist *music.Playlist) *SetPlayModeTool {
	return &SetPlayModeTool{playlist: playlist}
}

func (t *SetPlayModeTool) Name() string { return "set_play_mode" }

func (t *SetPlayModeTool) Description() string {
	return "设置音乐播放模式。当用户说'单曲循环'、'列表循环'、'顺序播放'等时使用。"
}

func (t *SetPlayModeTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"mode": {
				"type": "string",
				"description": "播放模式: sequence(顺序播放), loop(列表循环), single(单曲循环)",
				"enum": ["sequence", "loop", "single"]
			}
		},
		"required": ["mode"]
	}`)
}

func (t *SetPlayModeTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if t.playlist == nil {
		return `{"success":false,"message":"播放列表未初始化"}`, nil
	}

	var params struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	var mode music.PlayMode
	switch params.Mode {
	case "sequence":
		mode = music.PlayModeSequence
	case "loop":
		mode = music.PlayModeLoop
	case "single":
		mode = music.PlayModeSingle
	default:
		return `{"success":false,"message":"无效的播放模式，请选择 sequence/loop/single"}`, nil
	}

	t.playlist.SetMode(mode)
	return fmt.Sprintf(`{"success":true,"message":"已切换为%s模式"}`, mode), nil
}

// ---- ListMusicCacheTool 查看缓存列表 ----

type ListMusicCacheTool struct {
	cache *audio.MusicCache
}

func NewListMusicCacheTool(cache *audio.MusicCache) *ListMusicCacheTool {
	return &ListMusicCacheTool{cache: cache}
}

func (t *ListMusicCacheTool) Name() string { return "list_music_cache" }

func (t *ListMusicCacheTool) Description() string {
	return "查看本地缓存的音乐列表。当用户说'缓存了哪些歌'、'本地有什么歌'等时使用。"
}

func (t *ListMusicCacheTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`)
}

func (t *ListMusicCacheTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if t.cache == nil || !t.cache.Enabled() {
		return `{"success":false,"message":"音乐缓存未启用"}`, nil
	}

	entries := t.cache.List()
	if len(entries) == 0 {
		return `{"success":true,"message":"缓存为空，还没有缓存任何歌曲"}`, nil
	}

	result := fmt.Sprintf("本地缓存了 %d 首歌曲:\n", len(entries))
	for i, e := range entries {
		sizeKB := e.Size / 1024
		result += fmt.Sprintf("%d. %s - %s (%s, %dKB)\n", i+1, e.Name, e.Artist, e.Album, sizeKB)
	}
	return result, nil
}

// ---- DeleteMusicCacheTool 删除缓存音乐 ----

type DeleteMusicCacheTool struct {
	cache *audio.MusicCache
}

func NewDeleteMusicCacheTool(cache *audio.MusicCache) *DeleteMusicCacheTool {
	return &DeleteMusicCacheTool{cache: cache}
}

func (t *DeleteMusicCacheTool) Name() string { return "delete_music_cache" }

func (t *DeleteMusicCacheTool) Description() string {
	return "删除本地缓存的音乐。支持按关键词匹配歌名或歌手名，可选排除某些歌手。当用户说'删除缓存的某某歌'等时使用。"
}

func (t *DeleteMusicCacheTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"keyword": {
				"type": "string",
				"description": "要删除的歌曲关键词（歌名或歌手名）"
			},
			"exclude_artists": {
				"type": "array",
				"items": {"type": "string"},
				"description": "要保留（不删除）的歌手列表"
			}
		},
		"required": ["keyword"]
	}`)
}

func (t *DeleteMusicCacheTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if t.cache == nil || !t.cache.Enabled() {
		return `{"success":false,"message":"音乐缓存未启用"}`, nil
	}

	var params struct {
		Keyword        string   `json:"keyword"`
		ExcludeArtists []string `json:"exclude_artists"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	if params.Keyword == "" {
		return "", fmt.Errorf("缺少 keyword 参数")
	}

	deleted := t.cache.Delete(params.Keyword, params.ExcludeArtists)
	if deleted == 0 {
		return fmt.Sprintf(`{"success":true,"message":"没有找到匹配'%s'的缓存歌曲"}`, params.Keyword), nil
	}

	return fmt.Sprintf(`{"success":true,"message":"已删除 %d 首匹配'%s'的缓存歌曲"}`, deleted, params.Keyword), nil
}
