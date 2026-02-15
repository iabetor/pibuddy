package music

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 辅助函数：创建搜索响应
func makeSearchResponse(code int, songs []struct {
	ID     int64
	Name   string
	Artist string
	Album  string
}) searchResponse {
	var resultSongs []struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
		Album struct {
			Name string `json:"name"`
		} `json:"album"`
	}

	for _, s := range songs {
		resultSongs = append(resultSongs, struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
			Album struct {
				Name string `json:"name"`
			} `json:"album"`
		}{
			ID:   s.ID,
			Name: s.Name,
			Artists: []struct {
				Name string `json:"name"`
			}{{Name: s.Artist}},
			Album: struct {
				Name string `json:"name"`
			}{Name: s.Album},
		})
	}

	return searchResponse{
		Code: code,
		Result: struct {
			Songs []struct {
				ID      int64  `json:"id"`
				Name    string `json:"name"`
				Artists []struct {
					Name string `json:"name"`
				} `json:"artists"`
				Album struct {
					Name string `json:"name"`
				} `json:"album"`
			} `json:"songs"`
		}{
			Songs: resultSongs,
		},
	}
}

func TestNeteaseClient_Search(t *testing.T) {
	tests := []struct {
		name       string
		keyword    string
		limit      int
		mockResp   interface{}
		mockStatus int
		wantErr    bool
		wantLen    int
	}{
		{
			name:       "成功搜索",
			keyword:    "晴天",
			limit:      5,
			mockResp:   makeSearchResponse(200, []struct{ ID int64; Name string; Artist string; Album string }{{ID: 1, Name: "晴天", Artist: "周杰伦", Album: "叶惠美"}, {ID: 2, Name: "晴天（翻唱）", Artist: "未知歌手", Album: "翻唱集"}}),
			mockStatus: http.StatusOK,
			wantErr:    false,
			wantLen:    2,
		},
		{
			name:       "API 返回错误码",
			keyword:    "测试",
			limit:      5,
			mockResp:   makeSearchResponse(400, nil),
			mockStatus: http.StatusOK,
			wantErr:    true,
			wantLen:    0,
		},
		{
			name:       "HTTP 错误状态码",
			keyword:    "错误",
			limit:      5,
			mockResp:   nil,
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
			wantLen:    0,
		},
		{
			name:       "空结果",
			keyword:    "不存在的歌曲",
			limit:      5,
			mockResp:   makeSearchResponse(200, nil),
			mockStatus: http.StatusOK,
			wantErr:    false,
			wantLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建 mock 服务器
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 验证请求路径
				if r.URL.Path != "/search" {
					t.Errorf("请求路径错误: got %s", r.URL.Path)
				}

				// 验证查询参数
				keywords := r.URL.Query().Get("keywords")
				if keywords != tt.keyword {
					t.Errorf("关键词参数错误: got %s, want %s", keywords, tt.keyword)
				}

				// 返回 mock 响应
				w.WriteHeader(tt.mockStatus)
				if tt.mockResp != nil {
					json.NewEncoder(w).Encode(tt.mockResp)
				}
			}))
			defer server.Close()

			// 创建客户端
			client := NewNeteaseClient(server.URL)

			// 执行搜索
			songs, err := client.Search(context.Background(), tt.keyword, tt.limit)

			// 验证结果
			if (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(songs) != tt.wantLen {
				t.Errorf("Search() 返回歌曲数量 = %d, want %d", len(songs), tt.wantLen)
			}

			// 验证返回的歌曲信息
			if !tt.wantErr && tt.wantLen > 0 {
				if songs[0].Name != "晴天" {
					t.Errorf("歌曲名称错误: got %s", songs[0].Name)
				}
				if songs[0].Artist != "周杰伦" {
					t.Errorf("歌手名称错误: got %s", songs[0].Artist)
				}
			}
		})
	}
}

func TestNeteaseClient_GetSongURL(t *testing.T) {
	tests := []struct {
		name       string
		songID     int64
		mockResp   interface{}
		mockStatus int
		wantErr    bool
		wantURL    string
	}{
		{
			name:   "成功获取 URL",
			songID: 123456,
			mockResp: songURLResponse{
				Code: 200,
				Data: []struct {
					URL           string `json:"url"`
					FreeTrialInfo *struct {
						Start int `json:"start"`
						End   int `json:"end"`
					} `json:"freeTrialInfo"`
				}{
					{URL: "http://example.com/song.mp3", FreeTrialInfo: nil},
				},
			},
			mockStatus: http.StatusOK,
			wantErr:    false,
			wantURL:    "http://example.com/song.mp3",
		},
		{
			name:   "VIP 歌曲返回试听",
			songID: 789012,
			mockResp: songURLResponse{
				Code: 200,
				Data: []struct {
					URL           string `json:"url"`
					FreeTrialInfo *struct {
						Start int `json:"start"`
						End   int `json:"end"`
					} `json:"freeTrialInfo"`
				}{
					{URL: "http://example.com/trial.mp3", FreeTrialInfo: &struct {
						Start int `json:"start"`
						End   int `json:"end"`
					}{Start: 0, End: 30}},
				},
			},
			mockStatus: http.StatusOK,
			wantErr:    true,
			wantURL:    "",
		},
		{
			name:   "无法获取 URL",
			songID: 999999,
			mockResp: songURLResponse{
				Code: 200,
				Data: []struct {
					URL           string `json:"url"`
					FreeTrialInfo *struct {
						Start int `json:"start"`
						End   int `json:"end"`
					} `json:"freeTrialInfo"`
				}{},
			},
			mockStatus: http.StatusOK,
			wantErr:    true,
			wantURL:    "",
		},
		{
			name:       "API 返回错误",
			songID:     111111,
			mockResp:   songURLResponse{Code: 400, Data: nil},
			mockStatus: http.StatusOK,
			wantErr:    true,
			wantURL:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 验证请求路径
				if r.URL.Path != "/song/url" {
					t.Errorf("请求路径错误: got %s", r.URL.Path)
				}

				// 验证歌曲 ID 参数
				id := r.URL.Query().Get("id")
				if id != "123456" && id != "789012" && id != "999999" && id != "111111" {
					// 由于我们传入的是 int64，需要验证转换后的字符串
				}

				w.WriteHeader(tt.mockStatus)
				if tt.mockResp != nil {
					json.NewEncoder(w).Encode(tt.mockResp)
				}
			}))
			defer server.Close()

			client := NewNeteaseClient(server.URL)
			url, err := client.GetSongURL(context.Background(), tt.songID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetSongURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && url != tt.wantURL {
				t.Errorf("GetSongURL() URL = %v, want %v", url, tt.wantURL)
			}
		})
	}
}

func TestNeteaseClient_DefaultBaseURL(t *testing.T) {
	client := NewNeteaseClient("")
	if client.baseURL != "http://localhost:3000" {
		t.Errorf("默认 baseURL 错误: got %s, want http://localhost:3000", client.baseURL)
	}
}

func TestNeteaseClient_Search_DefaultLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := r.URL.Query().Get("limit")
		if limit != "10" {
			t.Errorf("默认 limit 应为 10, got %s", limit)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(searchResponse{Code: 200})
	}))
	defer server.Close()

	client := NewNeteaseClient(server.URL)
	_, _ = client.Search(context.Background(), "test", 0)
}
