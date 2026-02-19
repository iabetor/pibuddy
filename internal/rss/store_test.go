package rss

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFeedStoreAddAndList(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFeedStore(dir)
	if err != nil {
		t.Fatalf("NewFeedStore 失败: %v", err)
	}

	// 空列表
	if feeds := store.List(); len(feeds) != 0 {
		t.Fatalf("期望空列表，得到 %d 条", len(feeds))
	}

	// 添加
	feed := Feed{
		Name: "Test Feed",
		URL:  "https://example.com/feed.xml",
	}
	if err := store.Add(feed); err != nil {
		t.Fatalf("Add 失败: %v", err)
	}

	feeds := store.List()
	if len(feeds) != 1 {
		t.Fatalf("期望 1 条，得到 %d 条", len(feeds))
	}
	if feeds[0].Name != "Test Feed" {
		t.Errorf("名称不匹配: %s", feeds[0].Name)
	}
	if feeds[0].ID == "" {
		t.Error("ID 不应为空")
	}
	if feeds[0].AddedAt.IsZero() {
		t.Error("AddedAt 不应为零值")
	}
}

func TestFeedStoreAddDuplicate(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFeedStore(dir)
	if err != nil {
		t.Fatalf("NewFeedStore 失败: %v", err)
	}

	feed := Feed{Name: "Test", URL: "https://example.com/feed.xml"}
	if err := store.Add(feed); err != nil {
		t.Fatalf("第一次 Add 失败: %v", err)
	}

	// 重复添加
	if err := store.Add(feed); err == nil {
		t.Fatal("期望重复添加返回错误")
	}
}

func TestFeedStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFeedStore(dir)
	if err != nil {
		t.Fatalf("NewFeedStore 失败: %v", err)
	}

	feed := Feed{ID: "rss_001", Name: "Test Feed", URL: "https://example.com/feed.xml"}
	_ = store.Add(feed)

	// 按 ID 删除
	if !store.Delete("rss_001") {
		t.Fatal("按 ID 删除应成功")
	}
	if len(store.List()) != 0 {
		t.Fatal("删除后列表应为空")
	}

	// 删除不存在的
	if store.Delete("not_exist") {
		t.Fatal("删除不存在的应返回 false")
	}
}

func TestFeedStoreDeleteByName(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFeedStore(dir)
	if err != nil {
		t.Fatalf("NewFeedStore 失败: %v", err)
	}

	feed := Feed{ID: "rss_001", Name: "36氪", URL: "https://36kr.com/feed"}
	_ = store.Add(feed)

	// 按名称删除（不区分大小写）
	if !store.Delete("36氪") {
		t.Fatal("按名称删除应成功")
	}
}

func TestFeedStoreFindByName(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFeedStore(dir)
	if err != nil {
		t.Fatalf("NewFeedStore 失败: %v", err)
	}

	_ = store.Add(Feed{Name: "36氪科技", URL: "https://36kr.com/feed"})
	_ = store.Add(Feed{Name: "少数派", URL: "https://sspai.com/feed"})

	// 模糊查找
	f := store.FindByName("36氪")
	if f == nil {
		t.Fatal("应找到 36氪")
	}
	if f.Name != "36氪科技" {
		t.Errorf("名称不匹配: %s", f.Name)
	}

	// 查找不存在的
	if store.FindByName("不存在") != nil {
		t.Fatal("不应找到不存在的源")
	}
}

func TestFeedStorePersistence(t *testing.T) {
	dir := t.TempDir()

	// 第一次创建并添加
	store1, _ := NewFeedStore(dir)
	_ = store1.Add(Feed{ID: "rss_001", Name: "Test", URL: "https://example.com/feed"})

	// 确认文件存在
	if _, err := os.Stat(filepath.Join(dir, "rss_feeds.json")); err != nil {
		t.Fatalf("持久化文件不存在: %v", err)
	}

	// 第二次创建，应加载已有数据
	store2, _ := NewFeedStore(dir)
	feeds := store2.List()
	if len(feeds) != 1 {
		t.Fatalf("加载后期望 1 条，得到 %d 条", len(feeds))
	}
	if feeds[0].Name != "Test" {
		t.Errorf("加载后名称不匹配: %s", feeds[0].Name)
	}
}

func TestFeedStoreUpdateLastFetched(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFeedStore(dir)
	_ = store.Add(Feed{ID: "rss_001", Name: "Test", URL: "https://example.com/feed"})

	now := time.Now()
	store.UpdateLastFetched("rss_001", now)

	feeds := store.List()
	if feeds[0].LastFetched.IsZero() {
		t.Fatal("LastFetched 不应为零值")
	}
}

func TestFeedStoreConcurrency(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFeedStore(dir)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = store.Add(Feed{
				Name: "Test",
				URL:  "https://example.com/feed" + string(rune('0'+i)),
			})
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.List()
		}()
	}

	wg.Wait()
}
