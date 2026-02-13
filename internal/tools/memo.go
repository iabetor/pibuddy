package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MemoEntry 备忘录条目。
type MemoEntry struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Created string `json:"created"`
}

// MemoStore 备忘录持久化存储。
type MemoStore struct {
	mu       sync.RWMutex
	filePath string
	memos    []MemoEntry
}

// NewMemoStore 创建备忘录存储。
func NewMemoStore(dataDir string) (*MemoStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	s := &MemoStore{
		filePath: filepath.Join(dataDir, "memos.json"),
	}
	if err := s.load(); err != nil {
		log.Printf("[tools] 加载备忘录数据失败（将使用空列表）: %v", err)
		s.memos = make([]MemoEntry, 0)
	}
	return s, nil
}

func (s *MemoStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.memos = make([]MemoEntry, 0)
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &s.memos)
}

func (s *MemoStore) save() error {
	data, err := json.MarshalIndent(s.memos, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *MemoStore) Add(entry MemoEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memos = append(s.memos, entry)
	return s.save()
}

func (s *MemoStore) List() []MemoEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]MemoEntry, len(s.memos))
	copy(result, s.memos)
	return result
}

func (s *MemoStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, m := range s.memos {
		if m.ID == id {
			s.memos = append(s.memos[:i], s.memos[i+1:]...)
			_ = s.save()
			return true
		}
	}
	return false
}

// ---- AddMemoTool ----

type AddMemoTool struct {
	store *MemoStore
}

func NewAddMemoTool(store *MemoStore) *AddMemoTool {
	return &AddMemoTool{store: store}
}

func (t *AddMemoTool) Name() string { return "add_memo" }
func (t *AddMemoTool) Description() string {
	return "添加备忘录。当用户说'记一下'、'帮我备忘'等时使用。"
}
func (t *AddMemoTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"content": {
				"type": "string",
				"description": "备忘内容"
			}
		},
		"required": ["content"]
	}`)
}

type addMemoArgs struct {
	Content string `json:"content"`
}

func (t *AddMemoTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a addMemoArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	if a.Content == "" {
		return "", fmt.Errorf("备忘内容不能为空")
	}

	id := fmt.Sprintf("memo_%d", time.Now().UnixMilli())
	entry := MemoEntry{
		ID:      id,
		Content: a.Content,
		Created: time.Now().Format("2006-01-02 15:04:05"),
	}

	if err := t.store.Add(entry); err != nil {
		return "", fmt.Errorf("保存备忘录失败: %w", err)
	}

	return fmt.Sprintf("已记录备忘: %s", a.Content), nil
}

// ---- ListMemosTool ----

type ListMemosTool struct {
	store *MemoStore
}

func NewListMemosTool(store *MemoStore) *ListMemosTool {
	return &ListMemosTool{store: store}
}

func (t *ListMemosTool) Name() string { return "list_memos" }
func (t *ListMemosTool) Description() string {
	return "查看所有备忘录。当用户说'看看备忘'、'有哪些备忘'等时使用。"
}
func (t *ListMemosTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}

func (t *ListMemosTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	memos := t.store.List()
	if len(memos) == 0 {
		return "当前没有任何备忘录。", nil
	}
	result := fmt.Sprintf("当前有 %d 条备忘:\n", len(memos))
	for i, m := range memos {
		result += fmt.Sprintf("%d. [%s] %s (创建于 %s)\n", i+1, m.ID, m.Content, m.Created)
	}
	return result, nil
}

// ---- DeleteMemoTool ----

type DeleteMemoTool struct {
	store *MemoStore
}

func NewDeleteMemoTool(store *MemoStore) *DeleteMemoTool {
	return &DeleteMemoTool{store: store}
}

func (t *DeleteMemoTool) Name() string { return "delete_memo" }
func (t *DeleteMemoTool) Description() string {
	return "删除指定备忘录。当用户说'删除备忘'、'去掉那条备忘'等时使用。"
}
func (t *DeleteMemoTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {
				"type": "string",
				"description": "备忘录ID"
			}
		},
		"required": ["id"]
	}`)
}

type deleteMemoArgs struct {
	ID string `json:"id"`
}

func (t *DeleteMemoTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a deleteMemoArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	if t.store.Delete(a.ID) {
		return fmt.Sprintf("备忘录 %s 已删除", a.ID), nil
	}
	return fmt.Sprintf("未找到备忘录 %s", a.ID), nil
}
