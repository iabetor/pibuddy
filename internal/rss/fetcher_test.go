package rss

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Blog</title>
    <link>https://example.com</link>
    <description>A test RSS feed</description>
    <item>
      <title>第一篇文章</title>
      <link>https://example.com/post/1</link>
      <description>&lt;p&gt;这是第一篇文章的内容，包含 &lt;b&gt;HTML 标签&lt;/b&gt;。&lt;/p&gt;</description>
      <pubDate>Thu, 19 Feb 2026 08:00:00 +0800</pubDate>
    </item>
    <item>
      <title>AI 技术前沿</title>
      <link>https://example.com/post/2</link>
      <description>人工智能最新进展</description>
      <pubDate>Thu, 19 Feb 2026 07:00:00 +0800</pubDate>
    </item>
    <item>
      <title>第三篇普通文章</title>
      <link>https://example.com/post/3</link>
      <description>普通内容</description>
      <pubDate>Thu, 19 Feb 2026 06:00:00 +0800</pubDate>
    </item>
  </channel>
</rss>`

const testAtomFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Blog</title>
  <entry>
    <title>Atom 文章</title>
    <link href="https://example.com/atom/1"/>
    <summary>Atom 格式的摘要</summary>
    <updated>2026-02-19T09:00:00+08:00</updated>
  </entry>
</feed>`

func setupTestServer(content string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, content)
	}))
}

func TestFetchAndValidate(t *testing.T) {
	srv := setupTestServer(testRSSFeed)
	defer srv.Close()

	dir := t.TempDir()
	store, _ := NewFeedStore(dir)
	fetcher := NewFetcher(store, dir, 30)

	title, err := fetcher.FetchAndValidate(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchAndValidate 失败: %v", err)
	}
	if title != "Test Blog" {
		t.Errorf("标题不匹配: %s", title)
	}
}

func TestFetchAndValidateInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not xml")
	}))
	defer srv.Close()

	dir := t.TempDir()
	store, _ := NewFeedStore(dir)
	fetcher := NewFetcher(store, dir, 30)

	_, err := fetcher.FetchAndValidate(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("期望无效 Feed 返回错误")
	}
}

func TestFetchAndValidateAtom(t *testing.T) {
	srv := setupTestServer(testAtomFeed)
	defer srv.Close()

	dir := t.TempDir()
	store, _ := NewFeedStore(dir)
	fetcher := NewFetcher(store, dir, 30)

	title, err := fetcher.FetchAndValidate(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchAndValidate Atom 失败: %v", err)
	}
	if title != "Atom Blog" {
		t.Errorf("Atom 标题不匹配: %s", title)
	}
}

func TestGetNews(t *testing.T) {
	srv := setupTestServer(testRSSFeed)
	defer srv.Close()

	dir := t.TempDir()
	store, _ := NewFeedStore(dir)
	_ = store.Add(Feed{ID: "rss_001", Name: "Test Blog", URL: srv.URL})

	fetcher := NewFetcher(store, dir, 30)
	items, err := fetcher.GetNews(context.Background(), "", "", 5)
	if err != nil {
		t.Fatalf("GetNews 失败: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("期望 3 条，得到 %d 条", len(items))
	}

	// 应按时间倒序
	if items[0].Title != "第一篇文章" {
		t.Errorf("第一条应该是最新的: %s", items[0].Title)
	}
}

func TestGetNewsWithSourceFilter(t *testing.T) {
	rss1 := setupTestServer(testRSSFeed)
	defer rss1.Close()
	rss2 := setupTestServer(testAtomFeed)
	defer rss2.Close()

	dir := t.TempDir()
	store, _ := NewFeedStore(dir)
	_ = store.Add(Feed{ID: "rss_001", Name: "Test Blog", URL: rss1.URL})
	_ = store.Add(Feed{ID: "rss_002", Name: "Atom Blog", URL: rss2.URL})

	fetcher := NewFetcher(store, dir, 30)
	items, err := fetcher.GetNews(context.Background(), "Atom", "", 10)
	if err != nil {
		t.Fatalf("GetNews 按来源过滤失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("期望 1 条 Atom 内容，得到 %d 条", len(items))
	}
	if items[0].FeedName != "Atom Blog" {
		t.Errorf("FeedName 不匹配: %s", items[0].FeedName)
	}
}

func TestGetNewsWithKeywordFilter(t *testing.T) {
	srv := setupTestServer(testRSSFeed)
	defer srv.Close()

	dir := t.TempDir()
	store, _ := NewFeedStore(dir)
	_ = store.Add(Feed{ID: "rss_001", Name: "Test Blog", URL: srv.URL})

	fetcher := NewFetcher(store, dir, 30)
	items, err := fetcher.GetNews(context.Background(), "", "AI", 10)
	if err != nil {
		t.Fatalf("GetNews 按关键词过滤失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("期望 1 条含 AI 的内容，得到 %d 条", len(items))
	}
	if items[0].Title != "AI 技术前沿" {
		t.Errorf("标题不匹配: %s", items[0].Title)
	}
}

func TestGetNewsLimit(t *testing.T) {
	srv := setupTestServer(testRSSFeed)
	defer srv.Close()

	dir := t.TempDir()
	store, _ := NewFeedStore(dir)
	_ = store.Add(Feed{ID: "rss_001", Name: "Test Blog", URL: srv.URL})

	fetcher := NewFetcher(store, dir, 30)
	items, err := fetcher.GetNews(context.Background(), "", "", 2)
	if err != nil {
		t.Fatalf("GetNews 限制数量失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 条，得到 %d 条", len(items))
	}
}

func TestGetNewsNoFeeds(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFeedStore(dir)
	fetcher := NewFetcher(store, dir, 30)

	items, err := fetcher.GetNews(context.Background(), "", "", 5)
	if err != nil {
		t.Fatalf("无订阅源时不应返回错误: %v", err)
	}
	if items != nil {
		t.Fatal("无订阅源时应返回 nil")
	}
}

func TestCacheHit(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, testRSSFeed)
	}))
	defer srv.Close()

	dir := t.TempDir()
	store, _ := NewFeedStore(dir)
	_ = store.Add(Feed{ID: "rss_001", Name: "Test", URL: srv.URL})

	fetcher := NewFetcher(store, dir, 30)

	// 第一次请求
	_, _ = fetcher.GetNews(context.Background(), "", "", 5)
	if callCount != 1 {
		t.Fatalf("第一次请求应调用一次 HTTP，实际 %d 次", callCount)
	}

	// 第二次请求应使用缓存
	_, _ = fetcher.GetNews(context.Background(), "", "", 5)
	if callCount != 1 {
		t.Fatalf("第二次请求应使用缓存，实际 HTTP 调用 %d 次", callCount)
	}
}

func TestCacheExpired(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, testRSSFeed)
	}))
	defer srv.Close()

	dir := t.TempDir()
	store, _ := NewFeedStore(dir)
	_ = store.Add(Feed{ID: "rss_001", Name: "Test", URL: srv.URL})

	// 使用极短的缓存 TTL
	fetcher := NewFetcher(store, dir, 0)
	fetcher.cacheTTL = 1 * time.Millisecond

	_, _ = fetcher.GetNews(context.Background(), "", "", 5)
	time.Sleep(5 * time.Millisecond)
	_, _ = fetcher.GetNews(context.Background(), "", "", 5)

	if callCount != 2 {
		t.Fatalf("缓存过期后应重新请求，实际 HTTP 调用 %d 次", callCount)
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<p>Hello <b>World</b></p>", "Hello World"},
		{"plain text", "plain text"},
		{"&amp; &lt; &gt; &quot;", "& < > \""},
		{"<div>  多个   空格  </div>", "多个 空格"},
		{"", ""},
	}

	for _, tc := range tests {
		got := stripHTML(tc.input)
		if got != tc.expected {
			t.Errorf("stripHTML(%q) = %q, 期望 %q", tc.input, got, tc.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	// 短文本不截断
	short := "短文本"
	if got := truncate(short, 200); got != short {
		t.Errorf("短文本不应被截断: %s", got)
	}

	// 长文本截断
	long := ""
	for i := 0; i < 50; i++ {
		long += "这是一段很长的文字"
	}
	got := truncate(long, 200)
	runes := []rune(got)
	// 200 字符 + "..." = 203 runes
	if len(runes) != 203 {
		t.Errorf("截断后长度应为 203 rune，实际 %d", len(runes))
	}
}

func TestHTMLContentInFeed(t *testing.T) {
	srv := setupTestServer(testRSSFeed)
	defer srv.Close()

	dir := t.TempDir()
	store, _ := NewFeedStore(dir)
	_ = store.Add(Feed{ID: "rss_001", Name: "Test Blog", URL: srv.URL})

	fetcher := NewFetcher(store, dir, 30)
	items, _ := fetcher.GetNews(context.Background(), "", "", 5)

	// 第一条的 description 包含 HTML 标签，应该被剥离
	if items[0].Summary != "这是第一篇文章的内容，包含 HTML 标签。" {
		t.Errorf("HTML 应被剥离，实际: %s", items[0].Summary)
	}
}
