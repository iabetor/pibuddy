package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
)

// TimerEntry 倒计时条目。
type TimerEntry struct {
	ID        string `json:"id"`
	Duration  int    `json:"duration"`  // 总时长（秒）
	Remaining int    `json:"remaining"` // 剩余时间（秒）
	Label     string `json:"label"`     // 标签/提醒内容
	StartTime string `json:"start_time"`
	ExpiresAt string `json:"expires_at"`
}

// TimerStore 倒计时存储。
type TimerStore struct {
	mu        sync.RWMutex
	filePath  string
	timers    map[string]*TimerEntry
	timerMap  map[string]*time.Timer // Go timer 引用
	onExpire  func(entry TimerEntry) // 到期回调
	dataDir   string
}

// NewTimerStore 创建倒计时存储。
func NewTimerStore(dataDir string, onExpire func(entry TimerEntry)) (*TimerStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	s := &TimerStore{
		filePath: filepath.Join(dataDir, "timers.json"),
		timers:   make(map[string]*TimerEntry),
		timerMap: make(map[string]*time.Timer),
		onExpire: onExpire,
		dataDir:  dataDir,
	}
	if err := s.load(); err != nil {
		logger.Warnf("[tools] 加载倒计时数据失败（将使用空列表）: %v", err)
	}
	return s, nil
}

func (s *TimerStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var entries []TimerEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	// 恢复未过期的倒计时（不恢复 Go timer，因为已经重启了）
	now := time.Now()
	for _, e := range entries {
		expiresAt, err := time.Parse(time.RFC3339, e.ExpiresAt)
		if err != nil {
			continue
		}
		if expiresAt.After(now) {
			// 计算剩余时间
			e.Remaining = int(time.Until(expiresAt).Seconds())
			if e.Remaining > 0 {
				entry := e
				s.timers[entry.ID] = &entry
				// 重新启动 timer
				s.startTimer(&entry)
			}
		}
	}
	return nil
}

func (s *TimerStore) save() error {
	s.mu.RLock()
	entries := make([]TimerEntry, 0, len(s.timers))
	for _, e := range s.timers {
		entries = append(entries, *e)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// startTimer 启动 Go timer（必须在持有锁或初始化时调用）。
func (s *TimerStore) startTimer(entry *TimerEntry) {
	if s.onExpire == nil {
		return
	}

	duration := time.Duration(entry.Remaining) * time.Second
	timer := time.AfterFunc(duration, func() {
		s.mu.Lock()
		delete(s.timers, entry.ID)
		delete(s.timerMap, entry.ID)
		s.mu.Unlock()
		_ = s.save()

		// 调用回调
		s.onExpire(*entry)
	})

	s.timerMap[entry.ID] = timer
}

// Add 添加倒计时。
func (s *TimerStore) Add(entry *TimerEntry) error {
	s.mu.Lock()
	s.timers[entry.ID] = entry
	s.startTimer(entry)
	s.mu.Unlock()

	if err := s.save(); err != nil {
		return err
	}

	logger.Infof("[tools] 倒计时已启动: %s (%d秒)", entry.ID, entry.Duration)
	return nil
}

// List 列出所有倒计时。
func (s *TimerStore) List() []TimerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]TimerEntry, 0, len(s.timers))
	for _, e := range s.timers {
		// 更新剩余时间
		expiresAt, err := time.Parse(time.RFC3339, e.ExpiresAt)
		if err == nil {
			e.Remaining = int(time.Until(expiresAt).Seconds())
			if e.Remaining < 0 {
				e.Remaining = 0
			}
		}
		result = append(result, *e)
	}
	return result
}

// Cancel 取消倒计时。
func (s *TimerStore) Cancel(id string) bool {
	s.mu.Lock()
	timer, hasTimer := s.timerMap[id]
	if hasTimer {
		timer.Stop()
		delete(s.timerMap, id)
	}

	_, exists := s.timers[id]
	if exists {
		delete(s.timers, id)
	}
	s.mu.Unlock()

	if exists {
		_ = s.save()
		return true
	}
	return false
}

// Get 获取指定倒计时。
func (s *TimerStore) Get(id string) (*TimerEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.timers[id]
	return entry, ok
}

// ---- SetTimerTool ----

type SetTimerTool struct {
	store *TimerStore
}

func NewSetTimerTool(store *TimerStore) *SetTimerTool {
	return &SetTimerTool{store: store}
}

func (t *SetTimerTool) Name() string { return "set_timer" }
func (t *SetTimerTool) Description() string {
	return "设置倒计时器。当用户说'设个倒计时'、'提醒我'、'定时'等时使用。时间由LLM解析为秒数。"
}
func (t *SetTimerTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"duration_seconds": {
				"type": "integer",
				"description": "倒计时长（秒）"
			},
			"label": {
				"type": "string",
				"description": "提醒标签，如'关火'、'休息'"
			}
		},
		"required": ["duration_seconds"]
	}`)
}

type setTimerArgs struct {
	DurationSeconds int    `json:"duration_seconds"`
	Label           string `json:"label"`
}

func (t *SetTimerTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a setTimerArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	if a.DurationSeconds <= 0 {
		return "", fmt.Errorf("倒计时长必须大于0秒")
	}

	now := time.Now()
	id := fmt.Sprintf("timer_%d", now.UnixMilli())
	entry := &TimerEntry{
		ID:        id,
		Duration:  a.DurationSeconds,
		Remaining: a.DurationSeconds,
		Label:     a.Label,
		StartTime: now.Format(time.RFC3339),
		ExpiresAt: now.Add(time.Duration(a.DurationSeconds) * time.Second).Format(time.RFC3339),
	}

	if err := t.store.Add(entry); err != nil {
		return "", fmt.Errorf("保存倒计时失败: %w", err)
	}

	// 格式化时长
	durationStr := formatDuration(a.DurationSeconds)
	if a.Label != "" {
		return fmt.Sprintf("已设置%s倒计时，提醒内容：%s", durationStr, a.Label), nil
	}
	return fmt.Sprintf("已设置%s倒计时", durationStr), nil
}

// formatDuration 将秒数格式化为友好格式。
func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%d秒", seconds)
	} else if seconds < 3600 {
		mins := seconds / 60
		secs := seconds % 60
		if secs > 0 {
			return fmt.Sprintf("%d分%d秒", mins, secs)
		}
		return fmt.Sprintf("%d分钟", mins)
	} else {
		hours := seconds / 3600
		mins := (seconds % 3600) / 60
		if mins > 0 {
			return fmt.Sprintf("%d小时%d分钟", hours, mins)
		}
		return fmt.Sprintf("%d小时", hours)
	}
}

// ---- ListTimersTool ----

type ListTimersTool struct {
	store *TimerStore
}

func NewListTimersTool(store *TimerStore) *ListTimersTool {
	return &ListTimersTool{store: store}
}

func (t *ListTimersTool) Name() string { return "list_timers" }
func (t *ListTimersTool) Description() string {
	return "查看当前正在进行的倒计时。当用户说'有哪些倒计时'、'查看定时器'等时使用。"
}
func (t *ListTimersTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}

func (t *ListTimersTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	timers := t.store.List()
	if len(timers) == 0 {
		return "当前没有正在进行的倒计时。", nil
	}

	result := fmt.Sprintf("当前有 %d 个倒计时:\n", len(timers))
	for i, e := range timers {
		remaining := formatDuration(e.Remaining)
		if e.Label != "" {
			result += fmt.Sprintf("%d. [%s] 剩余%s - %s\n", i+1, e.ID, remaining, e.Label)
		} else {
			result += fmt.Sprintf("%d. [%s] 剩余%s\n", i+1, e.ID, remaining)
		}
	}
	return result, nil
}

// ---- CancelTimerTool ----

type CancelTimerTool struct {
	store *TimerStore
}

func NewCancelTimerTool(store *TimerStore) *CancelTimerTool {
	return &CancelTimerTool{store: store}
}

func (t *CancelTimerTool) Name() string { return "cancel_timer" }
func (t *CancelTimerTool) Description() string {
	return "取消倒计时。当用户说'取消倒计时'、'删除定时器'等时使用。可以通过ID或标签取消。"
}
func (t *CancelTimerTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {
				"type": "string",
				"description": "倒计时ID（可选）"
			},
			"label": {
				"type": "string",
				"description": "倒计时标签（可选，用于模糊匹配）"
			}
		},
		"required": []
	}`)
}

type cancelTimerArgs struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func (t *CancelTimerTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a cancelTimerArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// 如果提供了ID，直接取消
	if a.ID != "" {
		if t.store.Cancel(a.ID) {
			return fmt.Sprintf("倒计时 %s 已取消", a.ID), nil
		}
		return fmt.Sprintf("未找到倒计时 %s", a.ID), nil
	}

	// 如果提供了标签，尝试模糊匹配
	if a.Label != "" {
		timers := t.store.List()
		for _, e := range timers {
			if e.Label == a.Label {
				t.store.Cancel(e.ID)
				return fmt.Sprintf("已取消倒计时：%s", e.Label), nil
			}
		}
		return fmt.Sprintf("未找到标签为'%s'的倒计时", a.Label), nil
	}

	// 如果都没有提供，取消最近的一个
	timers := t.store.List()
	if len(timers) == 0 {
		return "当前没有倒计时可取消", nil
	}

	// 找到剩余时间最短的
	var minTimer *TimerEntry
	for i := range timers {
		if minTimer == nil || timers[i].Remaining < minTimer.Remaining {
			minTimer = &timers[i]
		}
	}
	if minTimer != nil {
		t.store.Cancel(minTimer.ID)
		if minTimer.Label != "" {
			return fmt.Sprintf("已取消倒计时：%s", minTimer.Label), nil
		}
		return fmt.Sprintf("已取消倒计时 %s", minTimer.ID), nil
	}

	return "取消失败", nil
}
