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

// AlarmEntry 闹钟条目。
type AlarmEntry struct {
	ID      string `json:"id"`
	Time    string `json:"time"`
	Message string `json:"message"`
	Created string `json:"created"`
}

// AlarmStore 闹钟持久化存储。
type AlarmStore struct {
	mu       sync.RWMutex
	filePath string
	alarms   []AlarmEntry
}

// NewAlarmStore 创建闹钟存储。
func NewAlarmStore(dataDir string) (*AlarmStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	s := &AlarmStore{
		filePath: filepath.Join(dataDir, "alarms.json"),
	}
	if err := s.load(); err != nil {
		log.Printf("[tools] 加载闹钟数据失败（将使用空列表）: %v", err)
		s.alarms = make([]AlarmEntry, 0)
	}
	return s, nil
}

func (s *AlarmStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.alarms = make([]AlarmEntry, 0)
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &s.alarms)
}

func (s *AlarmStore) save() error {
	data, err := json.MarshalIndent(s.alarms, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *AlarmStore) Add(entry AlarmEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alarms = append(s.alarms, entry)
	return s.save()
}

func (s *AlarmStore) List() []AlarmEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]AlarmEntry, len(s.alarms))
	copy(result, s.alarms)
	return result
}

func (s *AlarmStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.alarms {
		if a.ID == id {
			s.alarms = append(s.alarms[:i], s.alarms[i+1:]...)
			_ = s.save()
			return true
		}
	}
	return false
}

// PopDueAlarms 弹出所有到期闹钟。
func (s *AlarmStore) PopDueAlarms() []AlarmEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	var due []AlarmEntry
	var remaining []AlarmEntry
	for _, a := range s.alarms {
		t, err := time.ParseInLocation("2006-01-02 15:04", a.Time, time.Local)
		if err != nil {
			remaining = append(remaining, a)
			continue
		}
		if now.After(t) {
			due = append(due, a)
		} else {
			remaining = append(remaining, a)
		}
	}
	if len(due) > 0 {
		s.alarms = remaining
		_ = s.save()
	}
	return due
}

// ---- SetAlarmTool ----

type SetAlarmTool struct {
	store *AlarmStore
}

func NewSetAlarmTool(store *AlarmStore) *SetAlarmTool {
	return &SetAlarmTool{store: store}
}

func (t *SetAlarmTool) Name() string { return "set_alarm" }
func (t *SetAlarmTool) Description() string {
	return "设置闹钟或提醒。当用户说'提醒我'、'设个闹钟'等时使用。"
}
func (t *SetAlarmTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"time": {
				"type": "string",
				"description": "闹钟时间，格式为 YYYY-MM-DD HH:MM，例如 2026-02-13 14:30"
			},
			"message": {
				"type": "string",
				"description": "提醒内容"
			}
		},
		"required": ["time", "message"]
	}`)
}

type setAlarmArgs struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

func (t *SetAlarmTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a setAlarmArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// 验证时间格式
	parsedTime, err := time.ParseInLocation("2006-01-02 15:04", a.Time, time.Local)
	if err != nil {
		return "", fmt.Errorf("时间格式错误，应为 YYYY-MM-DD HH:MM: %w", err)
	}

	if time.Now().After(parsedTime) {
		return "", fmt.Errorf("闹钟时间不能是过去的时间")
	}

	id := fmt.Sprintf("alarm_%d", time.Now().UnixMilli())
	entry := AlarmEntry{
		ID:      id,
		Time:    a.Time,
		Message: a.Message,
		Created: time.Now().Format("2006-01-02 15:04:05"),
	}

	if err := t.store.Add(entry); err != nil {
		return "", fmt.Errorf("保存闹钟失败: %w", err)
	}

	return fmt.Sprintf("闹钟已设置: %s, 提醒内容: %s", a.Time, a.Message), nil
}

// ---- ListAlarmsTool ----

type ListAlarmsTool struct {
	store *AlarmStore
}

func NewListAlarmsTool(store *AlarmStore) *ListAlarmsTool {
	return &ListAlarmsTool{store: store}
}

func (t *ListAlarmsTool) Name() string { return "list_alarms" }
func (t *ListAlarmsTool) Description() string {
	return "查看当前设置的所有闹钟。当用户说'看看闹钟'、'有哪些提醒'等时使用。"
}
func (t *ListAlarmsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}

func (t *ListAlarmsTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	alarms := t.store.List()
	if len(alarms) == 0 {
		return "当前没有设置任何闹钟。", nil
	}
	result := fmt.Sprintf("当前有 %d 个闹钟:\n", len(alarms))
	for i, a := range alarms {
		result += fmt.Sprintf("%d. [%s] %s - %s\n", i+1, a.ID, a.Time, a.Message)
	}
	return result, nil
}

// ---- DeleteAlarmTool ----

type DeleteAlarmTool struct {
	store *AlarmStore
}

func NewDeleteAlarmTool(store *AlarmStore) *DeleteAlarmTool {
	return &DeleteAlarmTool{store: store}
}

func (t *DeleteAlarmTool) Name() string { return "delete_alarm" }
func (t *DeleteAlarmTool) Description() string {
	return "删除指定闹钟。当用户说'取消闹钟'、'删除提醒'等时使用。"
}
func (t *DeleteAlarmTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {
				"type": "string",
				"description": "闹钟ID"
			}
		},
		"required": ["id"]
	}`)
}

type deleteAlarmArgs struct {
	ID string `json:"id"`
}

func (t *DeleteAlarmTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a deleteAlarmArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	if t.store.Delete(a.ID) {
		return fmt.Sprintf("闹钟 %s 已删除", a.ID), nil
	}
	return fmt.Sprintf("未找到闹钟 %s", a.ID), nil
}
