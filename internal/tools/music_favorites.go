package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/iabetor/pibuddy/internal/llm"
	"github.com/iabetor/pibuddy/internal/music"
)

// FavoritesConfig 收藏工具配置。
type FavoritesConfig struct {
	Store          *music.FavoritesStore
	Playlist       *music.Playlist
	ContextManager *llm.ContextManager
}

// AddFavoriteTool 收藏歌曲工具。
type AddFavoriteTool struct {
	store          *music.FavoritesStore
	playlist       *music.Playlist
	contextManager *llm.ContextManager
}

// NewAddFavoriteTool 创建收藏工具。
func NewAddFavoriteTool(cfg FavoritesConfig) *AddFavoriteTool {
	return &AddFavoriteTool{
		store:          cfg.Store,
		playlist:       cfg.Playlist,
		contextManager: cfg.ContextManager,
	}
}

// Name 返回工具名称。
func (t *AddFavoriteTool) Name() string {
	return "add_favorite"
}

// Description 返回工具描述。
func (t *AddFavoriteTool) Description() string {
	return `收藏当前播放的歌曲到个人歌单。不同用户的收藏分开存储。`
}

// Parameters 返回工具参数定义。
func (t *AddFavoriteTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

// Execute 执行工具。
func (t *AddFavoriteTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	// 获取当前播放歌曲
	if t.playlist == nil {
		return `{"success":false,"message":"播放器未初始化"}`, nil
	}

	current := t.playlist.Current()
	if current == nil {
		return `{"success":false,"message":"当前没有正在播放的歌曲"}`, nil
	}

	// 获取当前用户
	userName := t.getCurrentSpeaker()

	// 从 Extra 中提取信息
	var mid, mediaMid, provider string
	if current.Song.Extra != nil {
		if v, ok := current.Song.Extra["mid"].(string); ok {
			mid = v
		}
		if v, ok := current.Song.Extra["media_mid"].(string); ok {
			mediaMid = v
		}
		if v, ok := current.Song.Extra["provider"].(string); ok {
			provider = v
		}
	}

	// 从 CacheKey 中提取 provider
	if provider == "" && current.CacheKey != "" {
		parts := strings.SplitN(current.CacheKey, "_", 2)
		if len(parts) > 0 {
			provider = parts[0]
		}
	}

	// 构造收藏歌曲信息
	song := music.FavoriteSong{
		ID:       current.Song.ID,
		MID:      mid,
		MediaMID: mediaMid,
		Name:     current.Song.Name,
		Artist:   current.Song.Artist,
		Album:    current.Song.Album,
		Provider: provider,
	}

	// 添加到收藏
	if err := t.store.Add(userName, song); err != nil {
		return fmt.Sprintf(`{"success":false,"message":"%s"}`, err.Error()), nil
	}

	return fmt.Sprintf(`{"success":true,"message":"已将《%s》添加到你的收藏"}`, song.Name), nil
}

// getCurrentSpeaker 获取当前说话人。
func (t *AddFavoriteTool) getCurrentSpeaker() string {
	if t.contextManager != nil {
		if name := t.contextManager.GetCurrentSpeaker(); name != "" {
			return name
		}
	}
	return "guest"
}

// RemoveFavoriteTool 删除收藏工具。
type RemoveFavoriteTool struct {
	store          *music.FavoritesStore
	playlist       *music.Playlist
	contextManager *llm.ContextManager
}

// NewRemoveFavoriteTool 创建删除收藏工具。
func NewRemoveFavoriteTool(cfg FavoritesConfig) *RemoveFavoriteTool {
	return &RemoveFavoriteTool{
		store:          cfg.Store,
		playlist:       cfg.Playlist,
		contextManager: cfg.ContextManager,
	}
}

// Name 返回工具名称。
func (t *RemoveFavoriteTool) Name() string {
	return "remove_favorite"
}

// Description 返回工具描述。
func (t *RemoveFavoriteTool) Description() string {
	return `从收藏列表中删除歌曲。删除当前播放的歌曲。`
}

// Parameters 返回工具参数定义。
func (t *RemoveFavoriteTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

// Execute 执行工具。
func (t *RemoveFavoriteTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	// 获取当前播放歌曲
	if t.playlist == nil {
		return `{"success":false,"message":"播放器未初始化"}`, nil
	}

	current := t.playlist.Current()
	if current == nil {
		return `{"success":false,"message":"当前没有正在播放的歌曲"}`, nil
	}

	// 获取当前用户
	userName := t.getCurrentSpeaker()

	// 从 Extra 中提取 provider
	var provider string
	if current.Song.Extra != nil {
		if v, ok := current.Song.Extra["provider"].(string); ok {
			provider = v
		}
	}
	if provider == "" && current.CacheKey != "" {
		parts := strings.SplitN(current.CacheKey, "_", 2)
		if len(parts) > 0 {
			provider = parts[0]
		}
	}

	// 从收藏中删除
	if err := t.store.Remove(userName, current.Song.ID, provider); err != nil {
		return fmt.Sprintf(`{"success":false,"message":"%s"}`, err.Error()), nil
	}

	return fmt.Sprintf(`{"success":true,"message":"已将《%s》从收藏中删除"}`, current.Song.Name), nil
}

// getCurrentSpeaker 获取当前说话人。
func (t *RemoveFavoriteTool) getCurrentSpeaker() string {
	if t.contextManager != nil {
		if name := t.contextManager.GetCurrentSpeaker(); name != "" {
			return name
		}
	}
	return "guest"
}

// ListFavoritesTool 列出收藏工具。
type ListFavoritesTool struct {
	store          *music.FavoritesStore
	contextManager *llm.ContextManager
}

// NewListFavoritesTool 创建列出收藏工具。
func NewListFavoritesTool(cfg FavoritesConfig) *ListFavoritesTool {
	return &ListFavoritesTool{
		store:          cfg.Store,
		contextManager: cfg.ContextManager,
	}
}

// Name 返回工具名称。
func (t *ListFavoritesTool) Name() string {
	return "list_favorites"
}

// Description 返回工具描述。
func (t *ListFavoritesTool) Description() string {
	return `列出用户收藏的歌曲。`
}

// Parameters 返回工具参数定义。
func (t *ListFavoritesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

// Execute 执行工具。
func (t *ListFavoritesTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	userName := "guest"
	if t.contextManager != nil {
		if name := t.contextManager.GetCurrentSpeaker(); name != "" {
			userName = name
		}
	}

	songs, err := t.store.List(userName)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"message":"获取收藏列表失败: %s"}`, err), nil
	}

	if len(songs) == 0 {
		return `{"success":true,"message":"你还没有收藏任何歌曲","songs":[]}`, nil
	}

	// 构造返回结果
	var songList []string
	for _, s := range songs {
		songList = append(songList, fmt.Sprintf("%s - %s", s.Name, s.Artist))
	}

	return fmt.Sprintf(`{"success":true,"message":"你收藏了%d首歌曲","songs":["%s"]}`,
		len(songs), strings.Join(songList, `","`)), nil
}

// PlayFavoritesTool 播放收藏工具。
type PlayFavoritesTool struct {
	store          *music.FavoritesStore
	playlist       *music.Playlist
	contextManager *llm.ContextManager
	provider       music.Provider
}

// NewPlayFavoritesTool 创建播放收藏工具。
func NewPlayFavoritesTool(cfg FavoritesConfig, provider music.Provider) *PlayFavoritesTool {
	return &PlayFavoritesTool{
		store:          cfg.Store,
		playlist:       cfg.Playlist,
		contextManager: cfg.ContextManager,
		provider:       provider,
	}
}

// Name 返回工具名称。
func (t *PlayFavoritesTool) Name() string {
	return "play_favorites"
}

// Description 返回工具描述。
func (t *PlayFavoritesTool) Description() string {
	return `播放用户收藏的歌曲。默认随机播放，可指定顺序播放。`
}

// Parameters 返回工具参数定义。
func (t *PlayFavoritesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"mode": {
				"type": "string",
				"enum": ["random", "sequential"],
				"description": "播放模式：random（随机播放）或 sequential（顺序播放），默认 random"
			}
		}
	}`)
}

// Execute 执行工具。
func (t *PlayFavoritesTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Mode string `json:"mode"`
	}
	json.Unmarshal(args, &params)

	if params.Mode == "" {
		params.Mode = "random"
	}

	// 获取当前用户
	userName := "guest"
	if t.contextManager != nil {
		if name := t.contextManager.GetCurrentSpeaker(); name != "" {
			userName = name
		}
	}

	// 获取收藏列表
	songs, err := t.store.List(userName)
	if err != nil {
		return `{"success":false,"message":"获取收藏列表失败"}`, nil
	}

	if len(songs) == 0 {
		return `{"success":false,"message":"你的收藏是空的，先收藏一些歌曲吧"}`, nil
	}

	// 随机或顺序排列
	if params.Mode == "random" {
		songs = shuffleSongs(songs)
	}

	// 转换为播放列表项
	items := make([]music.PlaylistItem, len(songs))
	for i, s := range songs {
		// 构造 Extra 字段
		extra := make(map[string]interface{})
		if s.MID != "" {
			extra["mid"] = s.MID
		}
		if s.MediaMID != "" {
			extra["media_mid"] = s.MediaMID
		}
		if s.Provider != "" {
			extra["provider"] = s.Provider
		}

		items[i] = music.PlaylistItem{
			Song: music.Song{
				ID:     s.ID,
				Name:   s.Name,
				Artist: s.Artist,
				Album:  s.Album,
				Extra:  extra,
			},
		}
	}

	// 替换播放列表
	t.playlist.Replace(items)

	return fmt.Sprintf(`{"success":true,"message":"正在播放你的收藏歌单，共%d首歌","count":%d}`, len(songs), len(songs)), nil
}

// shuffleSongs 随机打乱歌曲顺序。
func shuffleSongs(songs []music.FavoriteSong) []music.FavoriteSong {
	result := make([]music.FavoriteSong, len(songs))
	copy(result, songs)

	// Fisher-Yates 洗牌
	for i := len(result) - 1; i > 0; i-- {
		j := time.Now().Nanosecond() % (i + 1)
		result[i], result[j] = result[j], result[i]
	}

	return result
}
