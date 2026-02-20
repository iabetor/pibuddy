package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewsTool_Name(t *testing.T) {
	tool := NewNewsTool()
	if tool.Name() != "get_news" {
		t.Errorf("expected name 'get_news', got %q", tool.Name())
	}
}

// TestNewsTool_ParseResponse tests parsing of QQ News API response format
func TestNewsTool_ParseResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"ret": 0,
			"idlist": [{
				"newslist": [
					{"id": "header", "articletype": "560", "title": "热点标题"},
					{"id": "1001", "articletype": "0", "title": "新闻标题一"},
					{"id": "1002", "articletype": "0", "title": "新闻标题二"},
					{"id": "1003", "articletype": "0", "title": "新闻标题三"}
				]
			}]
		}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("failed to get mock server: %v", err)
	}
	defer resp.Body.Close()

	var qqResp qqNewsResp
	if err := json.NewDecoder(resp.Body).Decode(&qqResp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if qqResp.Ret != 0 {
		t.Errorf("expected ret=0, got %d", qqResp.Ret)
	}
	if len(qqResp.IDList) == 0 {
		t.Fatal("expected non-empty idlist")
	}

	newsList := qqResp.IDList[0].NewsList
	if len(newsList) != 4 {
		t.Errorf("expected 4 items, got %d", len(newsList))
	}

	// Verify articletype 560 is the header
	if newsList[0].ArticleType != "560" {
		t.Errorf("first item should be header type 560, got %q", newsList[0].ArticleType)
	}

	// Count actual news items (skip 560)
	count := 0
	for _, item := range newsList {
		if item.ArticleType != "560" && item.Title != "" {
			count++
		}
	}
	if count != 3 {
		t.Errorf("expected 3 news items, got %d", count)
	}
}

func TestNewsTool_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ret": 0, "idlist": [{"newslist": []}]}`)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("failed to get mock server: %v", err)
	}
	defer resp.Body.Close()

	var qqResp qqNewsResp
	json.NewDecoder(resp.Body).Decode(&qqResp)

	if len(qqResp.IDList[0].NewsList) != 0 {
		t.Errorf("expected empty newslist")
	}
}

func TestNewsTool_Parameters(t *testing.T) {
	tool := NewNewsTool()
	params := tool.Parameters()
	if !strings.Contains(string(params), "category") {
		t.Errorf("parameters should mention category, got %s", string(params))
	}
}
