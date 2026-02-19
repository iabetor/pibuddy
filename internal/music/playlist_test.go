package music

import (
	"context"
	"testing"
)

// mockProvider 用于测试的 mock provider
type mockProvider struct {
	urls map[int64]string
}

func (m *mockProvider) Search(ctx context.Context, keyword string, limit int) ([]Song, error) {
	return nil, nil
}

func (m *mockProvider) GetSongURL(ctx context.Context, songID int64) (string, error) {
	if url, ok := m.urls[songID]; ok {
		return url, nil
	}
	return "", nil
}

func newTestPlaylist() *Playlist {
	provider := &mockProvider{
		urls: map[int64]string{
			1: "http://example.com/song1.mp3",
			2: "http://example.com/song2.mp3",
			3: "http://example.com/song3.mp3",
		},
	}
	return NewPlaylist(provider, nil)
}

func TestPlaylist_SequenceMode(t *testing.T) {
	pl := newTestPlaylist()
	pl.Replace([]PlaylistItem{
		{Song: Song{ID: 1, Name: "歌曲1", Artist: "歌手A"}, URL: "http://example.com/song1.mp3"},
		{Song: Song{ID: 2, Name: "歌曲2", Artist: "歌手B"}, URL: "http://example.com/song2.mp3"},
		{Song: Song{ID: 3, Name: "歌曲3", Artist: "歌手C"}, URL: "http://example.com/song3.mp3"},
	})

	ctx := context.Background()

	// 应该按顺序播放 3 首
	url, name, _, ok := pl.Next(ctx)
	if !ok || url != "http://example.com/song1.mp3" || name != "歌曲1" {
		t.Fatalf("第1首: ok=%v, url=%s, name=%s", ok, url, name)
	}

	url, name, _, ok = pl.Next(ctx)
	if !ok || url != "http://example.com/song2.mp3" || name != "歌曲2" {
		t.Fatalf("第2首: ok=%v, url=%s, name=%s", ok, url, name)
	}

	url, name, _, ok = pl.Next(ctx)
	if !ok || url != "http://example.com/song3.mp3" || name != "歌曲3" {
		t.Fatalf("第3首: ok=%v, url=%s, name=%s", ok, url, name)
	}

	// 到末尾应该没有下一首
	_, _, _, ok = pl.Next(ctx)
	if ok {
		t.Fatal("顺序播放到末尾应返回 ok=false")
	}
}

func TestPlaylist_LoopMode(t *testing.T) {
	pl := newTestPlaylist()
	pl.SetMode(PlayModeLoop)
	pl.Replace([]PlaylistItem{
		{Song: Song{ID: 1, Name: "歌曲1"}, URL: "http://example.com/song1.mp3"},
		{Song: Song{ID: 2, Name: "歌曲2"}, URL: "http://example.com/song2.mp3"},
	})

	ctx := context.Background()

	// 播完2首后应该循环回到第1首
	pl.Next(ctx) // 歌曲1
	pl.Next(ctx) // 歌曲2

	url, name, _, ok := pl.Next(ctx) // 应该回到歌曲1
	if !ok || url != "http://example.com/song1.mp3" || name != "歌曲1" {
		t.Fatalf("循环模式第3次: ok=%v, url=%s, name=%s", ok, url, name)
	}
}

func TestPlaylist_SingleMode(t *testing.T) {
	pl := newTestPlaylist()
	pl.SetMode(PlayModeSingle)
	pl.Replace([]PlaylistItem{
		{Song: Song{ID: 1, Name: "歌曲1"}, URL: "http://example.com/song1.mp3"},
		{Song: Song{ID: 2, Name: "歌曲2"}, URL: "http://example.com/song2.mp3"},
	})

	ctx := context.Background()

	// 单曲循环应该一直播放同一首
	pl.Next(ctx) // 歌曲1
	url, name, _, ok := pl.Next(ctx)
	if !ok || url != "http://example.com/song1.mp3" || name != "歌曲1" {
		t.Fatalf("单曲循环第2次: ok=%v, url=%s, name=%s", ok, url, name)
	}

	url, name, _, ok = pl.Next(ctx)
	if !ok || url != "http://example.com/song1.mp3" || name != "歌曲1" {
		t.Fatalf("单曲循环第3次: ok=%v, url=%s, name=%s", ok, url, name)
	}
}

func TestPlaylist_HasNext(t *testing.T) {
	pl := newTestPlaylist()
	pl.Replace([]PlaylistItem{
		{Song: Song{ID: 1, Name: "歌曲1"}, URL: "http://example.com/song1.mp3"},
	})

	ctx := context.Background()

	if !pl.HasNext() {
		t.Fatal("应该有下一首")
	}

	pl.Next(ctx) // 播放唯一一首

	if pl.HasNext() {
		t.Fatal("顺序播放只有一首时，播放完后不应该有下一首")
	}

	// 切换到循环模式
	pl.SetMode(PlayModeLoop)
	if !pl.HasNext() {
		t.Fatal("循环模式下应该总有下一首")
	}
}

func TestPlaylist_EmptyList(t *testing.T) {
	pl := newTestPlaylist()
	ctx := context.Background()

	_, _, _, ok := pl.Next(ctx)
	if ok {
		t.Fatal("空列表应返回 ok=false")
	}

	if pl.HasNext() {
		t.Fatal("空列表 HasNext 应为 false")
	}
}

func TestPlaylist_ReplaceResetsIndex(t *testing.T) {
	pl := newTestPlaylist()
	pl.Replace([]PlaylistItem{
		{Song: Song{ID: 1, Name: "歌曲1"}, URL: "http://example.com/song1.mp3"},
	})
	ctx := context.Background()

	pl.Next(ctx) // 播放第一首
	if pl.CurrentIndex() != 0 {
		t.Fatalf("当前索引应为 0，实际 %d", pl.CurrentIndex())
	}

	// 替换后索引应重置
	pl.Replace([]PlaylistItem{
		{Song: Song{ID: 2, Name: "歌曲2"}, URL: "http://example.com/song2.mp3"},
	})
	if pl.CurrentIndex() != -1 {
		t.Fatalf("替换后索引应为 -1，实际 %d", pl.CurrentIndex())
	}
}
