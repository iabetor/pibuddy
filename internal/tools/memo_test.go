package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestMemoStore_CRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pibuddy-memo-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewMemoStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create memo store: %v", err)
	}

	// Initially empty
	memos := store.List()
	if len(memos) != 0 {
		t.Errorf("expected 0 memos, got %d", len(memos))
	}

	// Add
	if err := store.Add(MemoEntry{ID: "m1", Content: "test memo", Created: "2026-01-01"}); err != nil {
		t.Fatal(err)
	}

	memos = store.List()
	if len(memos) != 1 {
		t.Fatalf("expected 1 memo, got %d", len(memos))
	}
	if memos[0].Content != "test memo" {
		t.Errorf("expected content 'test memo', got %q", memos[0].Content)
	}

	// Delete
	if !store.Delete("m1") {
		t.Error("expected delete to return true")
	}
	if store.Delete("nonexistent") {
		t.Error("expected delete of nonexistent to return false")
	}

	memos = store.List()
	if len(memos) != 0 {
		t.Errorf("expected 0 memos after delete, got %d", len(memos))
	}
}

func TestMemoStore_Persistence(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "pibuddy-memo-persist-test")
	defer os.RemoveAll(tmpDir)

	store1, _ := NewMemoStore(tmpDir)
	store1.Add(MemoEntry{ID: "p1", Content: "persist", Created: "2026-01-01"})

	store2, _ := NewMemoStore(tmpDir)
	memos := store2.List()
	if len(memos) != 1 || memos[0].ID != "p1" {
		t.Errorf("persistence failed: got %v", memos)
	}
}

func TestAddMemoTool_Execute(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "pibuddy-addmemo-test")
	defer os.RemoveAll(tmpDir)

	store, _ := NewMemoStore(tmpDir)
	tool := NewAddMemoTool(store)

	if tool.Name() != "add_memo" {
		t.Errorf("expected name 'add_memo', got %q", tool.Name())
	}

	args, _ := json.Marshal(addMemoArgs{Content: "买牛奶"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "已记录备忘") {
		t.Errorf("result should contain '已记录备忘', got %q", result)
	}
	if !strings.Contains(result, "买牛奶") {
		t.Errorf("result should contain memo content, got %q", result)
	}

	// Verify stored
	memos := store.List()
	if len(memos) != 1 {
		t.Errorf("expected 1 memo, got %d", len(memos))
	}
}

func TestAddMemoTool_EmptyContent(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "pibuddy-addmemo-empty-test")
	defer os.RemoveAll(tmpDir)

	store, _ := NewMemoStore(tmpDir)
	tool := NewAddMemoTool(store)

	args, _ := json.Marshal(addMemoArgs{Content: ""})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestListMemosTool_Execute(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "pibuddy-listmemo-test")
	defer os.RemoveAll(tmpDir)

	store, _ := NewMemoStore(tmpDir)
	tool := NewListMemosTool(store)

	if tool.Name() != "list_memos" {
		t.Errorf("expected name 'list_memos', got %q", tool.Name())
	}

	// Empty
	result, _ := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if !strings.Contains(result, "没有任何备忘录") {
		t.Errorf("empty list should say no memos, got %q", result)
	}

	// With data
	store.Add(MemoEntry{ID: "m1", Content: "memo1", Created: "2026-01-01"})
	result, _ = tool.Execute(context.Background(), json.RawMessage(`{}`))
	if !strings.Contains(result, "1 条备忘") {
		t.Errorf("should say 1 memo, got %q", result)
	}
}

func TestDeleteMemoTool_Execute(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "pibuddy-delmemo-test")
	defer os.RemoveAll(tmpDir)

	store, _ := NewMemoStore(tmpDir)
	store.Add(MemoEntry{ID: "del1", Content: "to delete", Created: "2026-01-01"})

	tool := NewDeleteMemoTool(store)

	if tool.Name() != "delete_memo" {
		t.Errorf("expected name 'delete_memo', got %q", tool.Name())
	}

	args, _ := json.Marshal(deleteMemoArgs{ID: "del1"})
	result, _ := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "已删除") {
		t.Errorf("should confirm deletion, got %q", result)
	}

	result, _ = tool.Execute(context.Background(), args)
	if !strings.Contains(result, "未找到") {
		t.Errorf("should say not found, got %q", result)
	}
}
