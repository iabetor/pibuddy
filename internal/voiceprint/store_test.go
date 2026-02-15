package voiceprint

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// 验证数据库文件已创建
	dbPath := filepath.Join(dir, "voiceprint.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("database file not created at %s", dbPath)
	}
}

func TestAddUser(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	id, err := store.AddUser("alice")
	if err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	// 重复添加应返回相同 ID
	id2, err := store.AddUser("alice")
	if err != nil {
		t.Fatalf("AddUser (duplicate) failed: %v", err)
	}
	if id2 != id {
		t.Errorf("expected same ID %d for duplicate, got %d", id, id2)
	}
}

func TestGetUser(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	store.AddUser("bob")

	u, err := store.GetUser("bob")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if u == nil {
		t.Fatal("expected user, got nil")
	}
	if u.Name != "bob" {
		t.Errorf("expected name 'bob', got %q", u.Name)
	}

	// 不存在的用户
	u2, err := store.GetUser("nobody")
	if err != nil {
		t.Fatalf("GetUser (nonexistent) failed: %v", err)
	}
	if u2 != nil {
		t.Errorf("expected nil for nonexistent user, got %+v", u2)
	}
}

func TestListUsers(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	store.AddUser("alice")
	store.AddUser("bob")
	store.AddUser("charlie")

	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 3 {
		t.Errorf("expected 3 users, got %d", len(users))
	}
}

func TestAddAndGetEmbeddings(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	id, _ := store.AddUser("alice")
	emb := []float32{0.1, 0.2, 0.3, 0.4, 0.5}

	if err := store.AddEmbedding(id, emb); err != nil {
		t.Fatalf("AddEmbedding failed: %v", err)
	}

	all, err := store.GetAllEmbeddings()
	if err != nil {
		t.Fatalf("GetAllEmbeddings failed: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(all))
	}
	if all[0].UserName != "alice" {
		t.Errorf("expected user 'alice', got %q", all[0].UserName)
	}
	if len(all[0].Embedding) != len(emb) {
		t.Fatalf("embedding length mismatch: got %d, want %d", len(all[0].Embedding), len(emb))
	}
	for i, v := range emb {
		if math.Abs(float64(all[0].Embedding[i]-v)) > 1e-6 {
			t.Errorf("embedding[%d] = %f, want %f", i, all[0].Embedding[i], v)
		}
	}
}

func TestDeleteUserCascade(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	id, _ := store.AddUser("alice")
	store.AddEmbedding(id, []float32{0.1, 0.2, 0.3})
	store.AddEmbedding(id, []float32{0.4, 0.5, 0.6})

	// 添加另一个用户
	id2, _ := store.AddUser("bob")
	store.AddEmbedding(id2, []float32{0.7, 0.8, 0.9})

	// 删除 alice
	if err := store.DeleteUser("alice"); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	// alice 应该没了
	u, _ := store.GetUser("alice")
	if u != nil {
		t.Errorf("expected nil after delete, got %+v", u)
	}

	// alice 的 embeddings 应该被级联删除
	all, _ := store.GetAllEmbeddings()
	for _, ue := range all {
		if ue.UserName == "alice" {
			t.Errorf("expected alice embeddings to be deleted, but found one")
		}
	}

	// bob 还在
	if len(all) != 1 || all[0].UserName != "bob" {
		t.Errorf("expected only bob's embedding, got %v", all)
	}
}

func TestDeleteNonexistentUser(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	err := store.DeleteUser("nobody")
	if err == nil {
		t.Error("expected error when deleting nonexistent user")
	}
}

func TestFloat32Serialization(t *testing.T) {
	original := []float32{-1.0, 0, 0.5, 1.0, 3.14159, -0.001}
	blob := float32ToBytes(original)
	restored := bytesToFloat32(blob)

	if len(restored) != len(original) {
		t.Fatalf("length mismatch: got %d, want %d", len(restored), len(original))
	}
	for i, v := range original {
		if restored[i] != v {
			t.Errorf("[%d] = %f, want %f", i, restored[i], v)
		}
	}
}

func TestGetAllEmbeddingsMultipleUsers(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	id1, _ := store.AddUser("alice")
	store.AddEmbedding(id1, []float32{0.1, 0.2})
	store.AddEmbedding(id1, []float32{0.3, 0.4})

	id2, _ := store.AddUser("bob")
	store.AddEmbedding(id2, []float32{0.5, 0.6})

	all, err := store.GetAllEmbeddings()
	if err != nil {
		t.Fatalf("GetAllEmbeddings failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 embeddings total, got %d", len(all))
	}

	// 按用户统计
	counts := make(map[string]int)
	for _, ue := range all {
		counts[ue.UserName]++
	}
	if counts["alice"] != 2 {
		t.Errorf("expected 2 embeddings for alice, got %d", counts["alice"])
	}
	if counts["bob"] != 1 {
		t.Errorf("expected 1 embedding for bob, got %d", counts["bob"])
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	return store
}
