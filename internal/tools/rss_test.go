package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iabetor/pibuddy/internal/rss"
)

const testRSSXML = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Blog</title>
    <link>https://example.com</link>
    <item>
      <title>测试文章一</title>
      <link>https://example.com/1</link>
      <description>第一篇摘要</description>
      <pubDate>Thu, 19 Feb 2026 08:00:00 +0800</pubDate>
    </item>
    <item>
      <title>AI新闻</title>
      <link>https://example.com/2</link>
      <description>AI相关内容</description>
      <pubDate>Thu, 19 Feb 2026 07:00:00 +0800</pubDate>
    </item>
  </channel>
</rss>`

func setupRSSServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, testRSSXML)
	}))
}

func setupRSSTools(t *testing.T) (
	*AddRSSFeedTool,
	*ListRSSFeedsTool,
	*DeleteRSSFeedTool,
	*GetRSSNewsTool,
	*httptest.Server,
) {
	t.Helper()
	srv := setupRSSServer()

	dir := t.TempDir()
	store, err := rss.NewFeedStore(dir)
	if err != nil {
		t.Fatalf("NewFeedStore 失败: %v", err)
	}
	fetcher := rss.NewFetcher(store, dir, 30)

	return NewAddRSSFeedTool(store, fetcher),
		NewListRSSFeedsTool(store),
		NewDeleteRSSFeedTool(store),
		NewGetRSSNewsTool(store, fetcher),
		srv
}

func TestAddRSSFeedTool(t *testing.T) {
	addTool, listTool, _, _, srv := setupRSSTools(t)
	defer srv.Close()

	// 添加有效源
	args, _ := json.Marshal(map[string]string{"url": srv.URL})
	result, err := addTool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if result != "已成功订阅 Test Blog" {
		t.Errorf("结果不匹配: %s", result)
	}

	// 验证已添加
	listResult, _ := listTool.Execute(context.Background(), json.RawMessage(`{}`))
	if listResult == "当前没有任何 RSS 订阅。可以告诉我想订阅的网站 RSS 地址来添加。" {
		t.Error("添加后列表不应为空")
	}
}

func TestAddRSSFeedToolWithName(t *testing.T) {
	addTool, _, _, _, srv := setupRSSTools(t)
	defer srv.Close()

	args, _ := json.Marshal(map[string]string{"url": srv.URL, "name": "自定义名称"})
	result, err := addTool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if result != "已成功订阅 自定义名称" {
		t.Errorf("结果不匹配: %s", result)
	}
}

func TestAddRSSFeedToolDuplicate(t *testing.T) {
	addTool, _, _, _, srv := setupRSSTools(t)
	defer srv.Close()

	args, _ := json.Marshal(map[string]string{"url": srv.URL})
	_, _ = addTool.Execute(context.Background(), args)

	// 重复添加
	result, err := addTool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("重复添加不应返回 error: %v", err)
	}
	if result != "该订阅源已存在: Test Blog" {
		t.Errorf("重复添加结果不匹配: %s", result)
	}
}

func TestAddRSSFeedToolInvalidURL(t *testing.T) {
	addTool, _, _, _, srv := setupRSSTools(t)
	defer srv.Close()

	invalidSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not xml")
	}))
	defer invalidSrv.Close()

	args, _ := json.Marshal(map[string]string{"url": invalidSrv.URL})
	result, err := addTool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("无效 URL 不应返回 error: %v", err)
	}
	if result == "" {
		t.Error("无效 URL 应返回错误消息")
	}
}

func TestListRSSFeedsToolEmpty(t *testing.T) {
	_, listTool, _, _, srv := setupRSSTools(t)
	defer srv.Close()

	result, err := listTool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if result != "当前没有任何 RSS 订阅。可以告诉我想订阅的网站 RSS 地址来添加。" {
		t.Errorf("空列表结果不匹配: %s", result)
	}
}

func TestDeleteRSSFeedTool(t *testing.T) {
	addTool, _, deleteTool, _, srv := setupRSSTools(t)
	defer srv.Close()

	// 先添加
	args, _ := json.Marshal(map[string]string{"url": srv.URL})
	_, _ = addTool.Execute(context.Background(), args)

	// 按名称删除
	deleteArgs, _ := json.Marshal(map[string]string{"id": "Test Blog"})
	result, err := deleteTool.Execute(context.Background(), deleteArgs)
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if result != "已取消订阅 Test Blog" {
		t.Errorf("删除结果不匹配: %s", result)
	}
}

func TestDeleteRSSFeedToolNotFound(t *testing.T) {
	_, _, deleteTool, _, srv := setupRSSTools(t)
	defer srv.Close()

	args, _ := json.Marshal(map[string]string{"id": "not_exist"})
	result, _ := deleteTool.Execute(context.Background(), args)
	if result != "未找到订阅源 not_exist" {
		t.Errorf("未找到结果不匹配: %s", result)
	}
}

func TestGetRSSNewsTool(t *testing.T) {
	addTool, _, _, getNewsTool, srv := setupRSSTools(t)
	defer srv.Close()

	// 先添加
	args, _ := json.Marshal(map[string]string{"url": srv.URL})
	_, _ = addTool.Execute(context.Background(), args)

	// 获取内容
	newsArgs, _ := json.Marshal(map[string]interface{}{})
	result, err := getNewsTool.Execute(context.Background(), newsArgs)
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if result == "" {
		t.Error("结果不应为空")
	}
	if !contains(result, "测试文章一") {
		t.Errorf("结果应包含文章标题: %s", result)
	}
}

func TestGetRSSNewsToolNoFeeds(t *testing.T) {
	_, _, _, getNewsTool, srv := setupRSSTools(t)
	defer srv.Close()

	result, _ := getNewsTool.Execute(context.Background(), json.RawMessage(`{}`))
	if result != "你还没有添加任何 RSS 订阅，可以告诉我想订阅的网站地址。" {
		t.Errorf("无订阅结果不匹配: %s", result)
	}
}

func TestGetRSSNewsToolWithKeyword(t *testing.T) {
	addTool, _, _, getNewsTool, srv := setupRSSTools(t)
	defer srv.Close()

	args, _ := json.Marshal(map[string]string{"url": srv.URL})
	_, _ = addTool.Execute(context.Background(), args)

	newsArgs, _ := json.Marshal(map[string]string{"keyword": "AI"})
	result, _ := getNewsTool.Execute(context.Background(), newsArgs)
	if !contains(result, "AI新闻") {
		t.Errorf("关键词过滤结果应包含 AI 新闻: %s", result)
	}
	if contains(result, "测试文章一") {
		t.Errorf("关键词过滤结果不应包含无关文章: %s", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
