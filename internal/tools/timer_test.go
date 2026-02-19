package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestSetTimerTool(t *testing.T) {
	store, err := NewTimerStore(t.TempDir(), func(entry TimerEntry) {
		// 回调被调用
	})
	if err != nil {
		t.Fatalf("创建 TimerStore 失败: %v", err)
	}

	tool := NewSetTimerTool(store)

	// 测试设置 1 秒倒计时
	args := setTimerArgs{
		DurationSeconds: 1,
		Label:           "测试倒计时",
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Execute(context.Background(), argsJSON)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}

	if result == "" {
		t.Error("结果不应为空")
	}

	// 验证倒计时已添加
	timers := store.List()
	if len(timers) != 1 {
		t.Errorf("期望 1 个倒计时，实际 %d 个", len(timers))
	}
}

func TestListTimersTool(t *testing.T) {
	store, err := NewTimerStore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("创建 TimerStore 失败: %v", err)
	}

	tool := NewListTimersTool(store)

	// 空列表
	result, err := tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if result != "当前没有正在进行的倒计时。" {
		t.Errorf("期望空列表提示，实际: %s", result)
	}

	// 添加一个倒计时
	now := time.Now()
	store.Add(&TimerEntry{
		ID:        "timer_test",
		Duration:  60,
		Remaining: 60,
		Label:     "测试",
		StartTime: now.Format(time.RFC3339),
		ExpiresAt: now.Add(60 * time.Second).Format(time.RFC3339),
	})

	result, err = tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if result == "" {
		t.Error("结果不应为空")
	}
}

func TestCancelTimerTool(t *testing.T) {
	store, err := NewTimerStore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("创建 TimerStore 失败: %v", err)
	}

	tool := NewCancelTimerTool(store)

	// 添加一个倒计时（5秒，足够取消）
	now := time.Now()
	store.Add(&TimerEntry{
		ID:        "timer_cancel_test",
		Duration:  5,
		Remaining: 5,
		Label:     "测试取消",
		StartTime: now.Format(time.RFC3339),
		ExpiresAt: now.Add(5 * time.Second).Format(time.RFC3339),
	})

	// 取消倒计时
	args := cancelTimerArgs{ID: "timer_cancel_test"}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Execute(context.Background(), argsJSON)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if result == "" {
		t.Error("结果不应为空")
	}

	// 验证已取消
	timers := store.List()
	if len(timers) != 0 {
		t.Errorf("期望 0 个倒计时，实际 %d 个", len(timers))
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  int
		expected string
	}{
		{30, "30秒"},
		{60, "1分钟"},
		{90, "1分30秒"},
		{120, "2分钟"},
		{3600, "1小时"},
		{3661, "1小时1分钟"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.seconds)
		if result != tt.expected {
			t.Errorf("formatDuration(%d) = %s, 期望 %s", tt.seconds, result, tt.expected)
		}
	}
}
