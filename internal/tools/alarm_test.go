package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAlarmStore_CRUD(t *testing.T) {
	// Use temp dir
	tmpDir, err := os.MkdirTemp("", "pibuddy-alarm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewAlarmStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create alarm store: %v", err)
	}

	// Initially empty
	alarms := store.List()
	if len(alarms) != 0 {
		t.Errorf("expected 0 alarms, got %d", len(alarms))
	}

	// Add
	entry := AlarmEntry{
		ID:      "test_1",
		Time:    "2099-12-31 23:59",
		Message: "test alarm",
		Created: time.Now().Format("2006-01-02 15:04:05"),
	}
	if err := store.Add(entry); err != nil {
		t.Fatalf("failed to add alarm: %v", err)
	}

	alarms = store.List()
	if len(alarms) != 1 {
		t.Fatalf("expected 1 alarm, got %d", len(alarms))
	}
	if alarms[0].ID != "test_1" {
		t.Errorf("expected ID 'test_1', got %q", alarms[0].ID)
	}

	// Delete
	if !store.Delete("test_1") {
		t.Error("expected delete to return true")
	}
	if store.Delete("nonexistent") {
		t.Error("expected delete of nonexistent to return false")
	}

	alarms = store.List()
	if len(alarms) != 0 {
		t.Errorf("expected 0 alarms after delete, got %d", len(alarms))
	}
}

func TestAlarmStore_PopDueAlarms(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pibuddy-alarm-pop-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewAlarmStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add a past alarm and a future alarm
	past := AlarmEntry{
		ID:      "past_1",
		Time:    "2020-01-01 00:00",
		Message: "past alarm",
		Created: time.Now().Format("2006-01-02 15:04:05"),
	}
	future := AlarmEntry{
		ID:      "future_1",
		Time:    "2099-12-31 23:59",
		Message: "future alarm",
		Created: time.Now().Format("2006-01-02 15:04:05"),
	}
	store.Add(past)
	store.Add(future)

	due := store.PopDueAlarms()
	if len(due) != 1 {
		t.Fatalf("expected 1 due alarm, got %d", len(due))
	}
	if due[0].ID != "past_1" {
		t.Errorf("expected due alarm ID 'past_1', got %q", due[0].ID)
	}

	// Future alarm should remain
	remaining := store.List()
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining alarm, got %d", len(remaining))
	}
	if remaining[0].ID != "future_1" {
		t.Errorf("expected remaining alarm ID 'future_1', got %q", remaining[0].ID)
	}
}

func TestAlarmStore_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pibuddy-alarm-persist-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create and add
	store1, _ := NewAlarmStore(tmpDir)
	store1.Add(AlarmEntry{
		ID:      "persist_1",
		Time:    "2099-01-01 00:00",
		Message: "persist test",
		Created: time.Now().Format("2006-01-02 15:04:05"),
	})

	// Reload
	store2, _ := NewAlarmStore(tmpDir)
	alarms := store2.List()
	if len(alarms) != 1 {
		t.Fatalf("expected 1 alarm after reload, got %d", len(alarms))
	}
	if alarms[0].ID != "persist_1" {
		t.Errorf("expected ID 'persist_1' after reload, got %q", alarms[0].ID)
	}
}

func TestSetAlarmTool_Execute(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "pibuddy-setalarm-test")
	defer os.RemoveAll(tmpDir)

	store, _ := NewAlarmStore(tmpDir)
	tool := NewSetAlarmTool(store)

	if tool.Name() != "set_alarm" {
		t.Errorf("expected name 'set_alarm', got %q", tool.Name())
	}

	// Set a future alarm
	futureTime := time.Now().Add(24 * time.Hour).Format("2006-01-02 15:04")
	args, _ := json.Marshal(setAlarmArgs{
		Time:    futureTime,
		Message: "测试闹钟",
	})

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "闹钟已设置") {
		t.Errorf("result should contain '闹钟已设置', got %q", result)
	}

	// Verify stored
	alarms := store.List()
	if len(alarms) != 1 {
		t.Errorf("expected 1 alarm stored, got %d", len(alarms))
	}
}

func TestSetAlarmTool_PastTime(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "pibuddy-setalarm-past-test")
	defer os.RemoveAll(tmpDir)

	store, _ := NewAlarmStore(tmpDir)
	tool := NewSetAlarmTool(store)

	args, _ := json.Marshal(setAlarmArgs{
		Time:    "2020-01-01 00:00",
		Message: "past alarm",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for past time")
	}
}

func TestSetAlarmTool_BadTimeFormat(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "pibuddy-setalarm-badtime-test")
	defer os.RemoveAll(tmpDir)

	store, _ := NewAlarmStore(tmpDir)
	tool := NewSetAlarmTool(store)

	args, _ := json.Marshal(setAlarmArgs{
		Time:    "not-a-time",
		Message: "bad time",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for bad time format")
	}
}

func TestListAlarmsTool_Execute(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "pibuddy-listalarm-test")
	defer os.RemoveAll(tmpDir)

	store, _ := NewAlarmStore(tmpDir)
	tool := NewListAlarmsTool(store)

	if tool.Name() != "list_alarms" {
		t.Errorf("expected name 'list_alarms', got %q", tool.Name())
	}

	// Empty list
	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "没有设置任何闹钟") {
		t.Errorf("empty list should say no alarms, got %q", result)
	}

	// Add one
	store.Add(AlarmEntry{ID: "test", Time: "2099-01-01 00:00", Message: "test"})
	result, _ = tool.Execute(context.Background(), json.RawMessage(`{}`))
	if !strings.Contains(result, "1 个闹钟") {
		t.Errorf("should say 1 alarm, got %q", result)
	}
}

func TestDeleteAlarmTool_Execute(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "pibuddy-deletealarm-test")
	defer os.RemoveAll(tmpDir)

	store, _ := NewAlarmStore(tmpDir)
	store.Add(AlarmEntry{ID: "del_1", Time: "2099-01-01 00:00", Message: "to delete"})

	tool := NewDeleteAlarmTool(store)

	if tool.Name() != "delete_alarm" {
		t.Errorf("expected name 'delete_alarm', got %q", tool.Name())
	}

	args, _ := json.Marshal(deleteAlarmArgs{ID: "del_1"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "已删除") {
		t.Errorf("should confirm deletion, got %q", result)
	}

	// Delete again should say not found
	result, _ = tool.Execute(context.Background(), args)
	if !strings.Contains(result, "未找到") {
		t.Errorf("should say not found, got %q", result)
	}
}
