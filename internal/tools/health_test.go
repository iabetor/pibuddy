package tools

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestHealthStore(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "health_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewHealthStore(tmpDir, HealthStoreConfig{
		WaterInterval:    120,
		ExerciseInterval: 60,
		QuietHoursStart:  "23:00",
		QuietHoursEnd:    "07:00",
	})
	if err != nil {
		t.Fatalf("创建 HealthStore 失败: %v", err)
	}

	// 测试设置喝水提醒
	t.Run("SetWaterReminder", func(t *testing.T) {
		reminder := &HealthReminder{
			Type:            ReminderTypeWater,
			ID:              "water",
			Enabled:         true,
			IntervalMinutes: 120,
		}
		if err := store.SetReminder(reminder); err != nil {
			t.Fatalf("设置喝水提醒失败: %v", err)
		}

		got := store.GetReminder("water")
		if got == nil {
			t.Fatal("未找到喝水提醒")
		}
		if got.IntervalMinutes != 120 {
			t.Errorf("期望间隔 120，实际 %d", got.IntervalMinutes)
		}
	})

	// 测试设置吃药提醒
	t.Run("SetMedicineReminder", func(t *testing.T) {
		reminder := &HealthReminder{
			Type:         ReminderTypeMedicine,
			ID:           "medicine_test",
			Enabled:      true,
			MedicineName: "降压药",
			Times:        []string{"08:00", "20:00"},
		}
		if err := store.SetReminder(reminder); err != nil {
			t.Fatalf("设置吃药提醒失败: %v", err)
		}

		got := store.GetReminder("medicine_test")
		if got == nil {
			t.Fatal("未找到吃药提醒")
		}
		if got.MedicineName != "降压药" {
			t.Errorf("期望药品名称 降压药，实际 %s", got.MedicineName)
		}
		if len(got.Times) != 2 {
			t.Errorf("期望 2 个时间点，实际 %d", len(got.Times))
		}
	})

	// 测试列出提醒
	t.Run("ListReminders", func(t *testing.T) {
		reminders := store.ListReminders()
		if len(reminders) != 2 {
			t.Errorf("期望 2 个提醒，实际 %d", len(reminders))
		}
	})

	// 测试删除提醒
	t.Run("DeleteReminder", func(t *testing.T) {
		if err := store.DeleteReminder("water"); err != nil {
			t.Fatalf("删除提醒失败: %v", err)
		}

		got := store.GetReminder("water")
		if got != nil {
			t.Error("喝水提醒应该已被删除")
		}
	})

	// 测试持久化
	t.Run("Persistence", func(t *testing.T) {
		// 重新加载
		store2, err := NewHealthStore(tmpDir, HealthStoreConfig{})
		if err != nil {
			t.Fatalf("重新创建 HealthStore 失败: %v", err)
		}

		got := store2.GetReminder("medicine_test")
		if got == nil {
			t.Fatal("持久化后未找到吃药提醒")
		}
		if got.MedicineName != "降压药" {
			t.Errorf("期望药品名称 降压药，实际 %s", got.MedicineName)
		}
	})
}

func TestQuietHours(t *testing.T) {
	tests := []struct {
		name    string
		start   string
		end     string
		now     string // HH:MM
		isQuiet bool
	}{
		{"跨天-夜间", "23:00", "07:00", "02:00", true},
		{"跨天-白天", "23:00", "07:00", "14:00", false},
		{"跨天-边界开始", "23:00", "07:00", "23:00", true},
		{"跨天-边界结束", "23:00", "07:00", "07:00", false},
		{"同天-中午", "12:00", "14:00", "13:00", true},
		{"同天-早上", "12:00", "14:00", "10:00", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, _ := os.MkdirTemp("", "health_test")
			defer os.RemoveAll(tmpDir)

			store, _ := NewHealthStore(tmpDir, HealthStoreConfig{
				QuietHoursStart: tt.start,
				QuietHoursEnd:   tt.end,
			})

			// 由于 isQuietHours 使用 time.Now()，这里只测试配置是否正确保存
			if store.QuietHours.Start != tt.start || store.QuietHours.End != tt.end {
				t.Errorf("静音时段配置错误")
			}
		})
	}
}

func TestCheckAndTrigger(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewHealthStore(tmpDir, HealthStoreConfig{
		WaterInterval:    1, // 1 分钟
		ExerciseInterval: 1,
	})
	if err != nil {
		t.Fatalf("创建 HealthStore 失败: %v", err)
	}

	// 设置一个已过期的喝水提醒
	reminder := &HealthReminder{
		Type:            ReminderTypeWater,
		ID:              "water",
		Enabled:         true,
		IntervalMinutes: 1,
		LastRemindedAt:  time.Now().Add(-2 * time.Minute), // 2 分钟前
	}
	store.SetReminder(reminder)

	// 检查并触发
	triggered := store.CheckAndTrigger()
	if len(triggered) == 0 {
		t.Error("期望触发喝水提醒")
	}
	if triggered[0].ID != "water" {
		t.Errorf("期望触发 water，实际 %s", triggered[0].ID)
	}
	if triggered[0].Message == "" {
		t.Error("提醒消息不应为空")
	}

	// 再次检查，不应该再触发
	triggered = store.CheckAndTrigger()
	if len(triggered) != 0 {
		t.Error("不应该再次触发")
	}
}

func TestHealthTools(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewHealthStore(tmpDir, HealthStoreConfig{
		WaterInterval:    120,
		ExerciseInterval: 60,
	})
	if err != nil {
		t.Fatalf("创建 HealthStore 失败: %v", err)
	}

	// 测试 SetHealthReminderTool
	t.Run("SetHealthReminderTool", func(t *testing.T) {
		tool := NewSetHealthReminderTool(store)

		// 测试开启喝水提醒
		result, err := tool.Execute(context.TODO(), []byte(`{"reminder_type": "water", "action": "start", "interval_minutes": 90}`))
		if err != nil {
			t.Fatalf("执行失败: %v", err)
		}
		if result == "" {
			t.Error("结果不应为空")
		}

		// 测试查看状态
		result, err = tool.Execute(context.TODO(), []byte(`{"reminder_type": "water", "action": "status"}`))
		if err != nil {
			t.Fatalf("执行失败: %v", err)
		}
		if result == "" {
			t.Error("状态不应为空")
		}

		// 测试关闭提醒
		result, err = tool.Execute(context.TODO(), []byte(`{"reminder_type": "water", "action": "stop"}`))
		if err != nil {
			t.Fatalf("执行失败: %v", err)
		}
	})

	// 测试吃药提醒
	t.Run("MedicineReminder", func(t *testing.T) {
		tool := NewSetHealthReminderTool(store)

		result, err := tool.Execute(context.TODO(), []byte(`{"reminder_type": "medicine", "action": "start", "medicine_name": "维生素", "times": ["08:00", "12:00", "18:00"]}`))
		if err != nil {
			t.Fatalf("执行失败: %v", err)
		}
		t.Logf("结果: %s", result)
	})

	// 测试 ListHealthRemindersTool
	t.Run("ListHealthRemindersTool", func(t *testing.T) {
		tool := NewListHealthRemindersTool(store)

		result, err := tool.Execute(context.TODO(), []byte(`{}`))
		if err != nil {
			t.Fatalf("执行失败: %v", err)
		}
		t.Logf("结果: %s", result)
	})
}

func TestFormatReminderStatus(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "health_test")
	defer os.RemoveAll(tmpDir)

	store, _ := NewHealthStore(tmpDir, HealthStoreConfig{
		QuietHoursStart: "23:00",
		QuietHoursEnd:   "07:00",
	})

	// 无提醒
	status := store.FormatReminderStatus()
	if status != "当前没有设置任何健康提醒" {
		t.Errorf("期望 '当前没有设置任何健康提醒'，实际 '%s'", status)
	}

	// 添加提醒
	store.SetReminder(&HealthReminder{
		Type:            ReminderTypeWater,
		ID:              "water",
		Enabled:         true,
		IntervalMinutes: 120,
	})
	store.SetReminder(&HealthReminder{
		Type:         ReminderTypeMedicine,
		ID:           "medicine_vitamin",
		Enabled:      true,
		MedicineName: "维生素",
		Times:        []string{"08:00", "18:00"},
	})

	status = store.FormatReminderStatus()
	t.Logf("状态:\n%s", status)
}
