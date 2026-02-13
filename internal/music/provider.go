package music

import "context"

// Song 表示一首歌曲的基本信息。
type Song struct {
	ID     int64  // 歌曲ID
	Name   string // 歌曲名
	Artist string // 歌手名
	Album  string // 专辑名
}

// Provider 定义音乐服务提供者接口。
type Provider interface {
	// Search 根据关键词搜索歌曲。
	Search(ctx context.Context, keyword string, limit int) ([]Song, error)

	// GetSongURL 获取歌曲播放地址。
	GetSongURL(ctx context.Context, songID int64) (string, error)
}
