package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/iabetor/pibuddy/internal/music"
)

// MockProvider 实现 music.Provider 接口用于测试
type MockProvider struct {
	searchResult []music.Song
	searchErr    error
	urlResult    string
	urlErr       error
}

func (m *MockProvider) Search(ctx context.Context, keyword string, limit int) ([]music.Song, error) {
	return m.searchResult, m.searchErr
}

func (m *MockProvider) GetSongURL(ctx context.Context, songID int64) (string, error) {
	return m.urlResult, m.urlErr
}

func (m *MockProvider) ProviderName() string { return "mock" }

func TestSearchMusicTool_Execute(t *testing.T) {
	tests := []struct {
		name       string
		provider   music.Provider
		enabled    bool
		args       string
		wantErr    bool
		wantCount  int
		wantMsg    string
	}{
		{
			name:     "成功搜索",
			provider: &MockProvider{searchResult: []music.Song{{ID: 1, Name: "晴天", Artist: "周杰伦", Album: "叶惠美"}}},
			enabled:  true,
			args:     `{"keyword": "晴天"}`,
			wantErr:  false,
			wantCount: 1,
		},
		{
			name:     "服务未启用",
			provider: nil,
			enabled:  false,
			args:     `{"keyword": "晴天"}`,
			wantErr:  false,
			wantMsg:  "音乐服务未启用",
		},
		{
			name:     "无结果",
			provider: &MockProvider{searchResult: []music.Song{}},
			enabled:  true,
			args:     `{"keyword": "不存在的歌"}`,
			wantErr:  false,
			wantMsg:  "没有找到相关歌曲",
		},
		{
			name:     "缺少关键词",
			provider: &MockProvider{},
			enabled:  true,
			args:     `{"keyword": ""}`,
			wantErr:  true,
		},
		{
			name:     "无效JSON",
			provider: &MockProvider{},
			enabled:  true,
			args:     `invalid json`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewSearchMusicTool(MusicConfig{
				Provider: tt.provider,
				Enabled:  tt.enabled,
			})

			result, err := tool.Execute(context.Background(), json.RawMessage(tt.args))

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				var searchResult SearchResult
				if err := json.Unmarshal([]byte(result), &searchResult); err != nil {
					t.Errorf("解析结果失败: %v", err)
					return
				}

				if tt.wantMsg != "" && searchResult.Error == "" {
					t.Errorf("期望错误消息包含 %q, 但结果为空", tt.wantMsg)
				}

				if tt.wantCount > 0 && len(searchResult.Songs) != tt.wantCount {
					t.Errorf("返回歌曲数量 = %d, want %d", len(searchResult.Songs), tt.wantCount)
				}
			}
		})
	}
}

func TestPlayMusicTool_Execute(t *testing.T) {
	tests := []struct {
		name     string
		provider music.Provider
		enabled  bool
		args     string
		wantErr  bool
		wantURL  string
		wantMsg  string
	}{
		{
			name: "成功播放",
			provider: &MockProvider{
				searchResult: []music.Song{{ID: 1, Name: "晴天", Artist: "周杰伦", Album: "叶惠美"}},
				urlResult:    "http://example.com/song.mp3",
			},
			enabled: true,
			args:    `{"keyword": "周杰伦晴天"}`,
			wantErr: false,
			wantURL: "http://example.com/song.mp3",
		},
		{
			name:     "服务未启用",
			provider: nil,
			enabled:  false,
			args:     `{"keyword": "晴天"}`,
			wantErr:  false,
			wantMsg:  "音乐服务未启用",
		},
		{
			name: "搜索无结果",
			provider: &MockProvider{
				searchResult: []music.Song{},
			},
			enabled: true,
			args:    `{"keyword": "不存在的歌"}`,
			wantErr: false,
			wantMsg: "没有找到相关歌曲",
		},
		{
			name:     "缺少关键词",
			provider: &MockProvider{},
			enabled:  true,
			args:     `{"keyword": ""}`,
			wantErr:  true,
		},
		{
			name:     "无效JSON",
			provider: &MockProvider{},
			enabled:  true,
			args:     `invalid json`,
			wantErr:  true,
		},
		{
			name: "所有歌曲无法播放则 fallback",
			provider: &MockProvider{
				searchResult: []music.Song{
					{ID: 1, Name: "晴天", Artist: "周杰伦"},
					{ID: 2, Name: "夜曲", Artist: "周杰伦"},
				},
				urlErr: fmt.Errorf("VIP 歌曲"),
			},
			enabled: true,
			args:    `{"keyword": "周杰伦"}`,
			wantErr: false,
			wantMsg: "均因版权限制无法播放",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewPlayMusicTool(MusicConfig{
				Provider: tt.provider,
				Enabled:  tt.enabled,
			})

			result, err := tool.Execute(context.Background(), json.RawMessage(tt.args))

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				var musicResult MusicResult
				if err := json.Unmarshal([]byte(result), &musicResult); err != nil {
					t.Errorf("解析结果失败: %v", err)
					return
				}

				if tt.wantURL != "" && musicResult.URL != tt.wantURL {
					t.Errorf("URL = %v, want %v", musicResult.URL, tt.wantURL)
				}

				if tt.wantMsg != "" && musicResult.Error == "" {
					t.Errorf("期望错误消息包含 %q, 但结果为空", tt.wantMsg)
				}
			}
		})
	}
}

func TestListMusicHistoryTool_Execute(t *testing.T) {
	tests := []struct {
		name    string
		history *music.HistoryStore
		args    string
		wantMsg string
	}{
		{
			name:    "无历史记录",
			history: nil,
			args:    `{}`,
			wantMsg: "播放历史功能未启用",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewListMusicHistoryTool(tt.history)

			result, err := tool.Execute(context.Background(), json.RawMessage(tt.args))
			if err != nil {
				t.Errorf("Execute() unexpected error: %v", err)
				return
			}

			if tt.wantMsg != "" && result != tt.wantMsg {
				t.Errorf("结果 = %q, want %q", result, tt.wantMsg)
			}
		})
	}
}

func TestMusicTool_Metadata(t *testing.T) {
	t.Run("SearchMusicTool", func(t *testing.T) {
		tool := NewSearchMusicTool(MusicConfig{})
		if tool.Name() != "search_music" {
			t.Errorf("Name() = %s, want search_music", tool.Name())
		}
		if tool.Description() == "" {
			t.Error("Description() 不应为空")
		}
		if len(tool.Parameters()) == 0 {
			t.Error("Parameters() 不应为空")
		}
	})

	t.Run("PlayMusicTool", func(t *testing.T) {
		tool := NewPlayMusicTool(MusicConfig{})
		if tool.Name() != "play_music" {
			t.Errorf("Name() = %s, want play_music", tool.Name())
		}
		if tool.Description() == "" {
			t.Error("Description() 不应为空")
		}
		if len(tool.Parameters()) == 0 {
			t.Error("Parameters() 不应为空")
		}
	})

	t.Run("ListMusicHistoryTool", func(t *testing.T) {
		tool := NewListMusicHistoryTool(nil)
		if tool.Name() != "list_music_history" {
			t.Errorf("Name() = %s, want list_music_history", tool.Name())
		}
		if tool.Description() == "" {
			t.Error("Description() 不应为空")
		}
		if len(tool.Parameters()) == 0 {
			t.Error("Parameters() 不应为空")
		}
	})
}
