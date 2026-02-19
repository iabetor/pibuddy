package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SetHealthReminderTool 设置健康提醒工具。
type SetHealthReminderTool struct {
	store *HealthStore
}

// NewSetHealthReminderTool 创建设置健康提醒工具。
func NewSetHealthReminderTool(store *HealthStore) *SetHealthReminderTool {
	return &SetHealthReminderTool{store: store}
}

// Name 返回工具名称。
func (t *SetHealthReminderTool) Name() string {
	return "set_health_reminder"
}

// Description 返回工具描述。
func (t *SetHealthReminderTool) Description() string {
	return `设置健康提醒。支持三种类型：
1. water - 喝水提醒，按间隔时间提醒
2. exercise - 久坐提醒，按间隔时间提醒
3. medicine - 吃药提醒，按每天固定时间提醒

操作 action:
- start: 开启提醒
- stop: 关闭提醒
- status: 查看当前提醒状态`
}

// Parameters 返回工具参数定义。
func (t *SetHealthReminderTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"reminder_type": {
				"type": "string",
				"enum": ["water", "exercise", "medicine"],
				"description": "提醒类型：water(喝水)、exercise(久坐)、medicine(吃药)"
			},
			"action": {
				"type": "string",
				"enum": ["start", "stop", "status"],
				"description": "操作：start(开启)、stop(关闭)、status(查看状态)"
			},
			"interval_minutes": {
				"type": "integer",
				"description": "提醒间隔（分钟），用于 water 和 exercise 类型。默认喝水120分钟，久坐60分钟"
			},
			"medicine_name": {
				"type": "string",
				"description": "药品名称，用于 medicine 类型"
			},
			"times": {
				"type": "array",
				"items": {"type": "string"},
				"description": "提醒时间列表，用于 medicine 类型，格式如 [\"08:00\", \"20:00\"]"
			}
		},
		"required": ["reminder_type", "action"]
	}`)
}

// Execute 执行工具。
func (t *SetHealthReminderTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		ReminderType    string   `json:"reminder_type"`
		Action          string   `json:"action"`
		IntervalMinutes int      `json:"interval_minutes"`
		MedicineName    string   `json:"medicine_name"`
		Times           []string `json:"times"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	switch params.Action {
	case "status":
		return t.store.FormatReminderStatus(), nil

	case "start":
		return t.startReminder(params)

	case "stop":
		return t.stopReminder(params)

	default:
		return "", fmt.Errorf("不支持的操作: %s", params.Action)
	}
}

// startReminder 开启提醒。
func (t *SetHealthReminderTool) startReminder(params struct {
	ReminderType    string   `json:"reminder_type"`
	Action          string   `json:"action"`
	IntervalMinutes int      `json:"interval_minutes"`
	MedicineName    string   `json:"medicine_name"`
	Times           []string `json:"times"`
}) (string, error) {
	reminderType := ReminderType(params.ReminderType)

	switch reminderType {
	case ReminderTypeWater:
		interval := params.IntervalMinutes
		if interval <= 0 {
			interval = 120 // 默认 2 小时
		}

		reminder := &HealthReminder{
			Type:            ReminderTypeWater,
			ID:              "water",
			Enabled:         true,
			IntervalMinutes: interval,
		}

		if err := t.store.SetReminder(reminder); err != nil {
			return "", fmt.Errorf("设置提醒失败: %w", err)
		}

		return fmt.Sprintf("已开启喝水提醒，每%d分钟提醒一次", interval), nil

	case ReminderTypeExercise:
		interval := params.IntervalMinutes
		if interval <= 0 {
			interval = 60 // 默认 1 小时
		}

		reminder := &HealthReminder{
			Type:            ReminderTypeExercise,
			ID:              "exercise",
			Enabled:         true,
			IntervalMinutes: interval,
		}

		if err := t.store.SetReminder(reminder); err != nil {
			return "", fmt.Errorf("设置提醒失败: %w", err)
		}

		return fmt.Sprintf("已开启久坐提醒，每%d分钟提醒一次", interval), nil

	case ReminderTypeMedicine:
		if params.MedicineName == "" {
			return "", fmt.Errorf("吃药提醒需要提供药品名称")
		}
		if len(params.Times) == 0 {
			return "", fmt.Errorf("吃药提醒需要提供提醒时间")
		}

		// 验证时间格式
		for _, t := range params.Times {
			if len(t) != 5 || t[2] != ':' {
				return "", fmt.Errorf("时间格式错误，应为 HH:MM 格式，如 08:00")
			}
		}

		reminder := &HealthReminder{
			Type:         ReminderTypeMedicine,
			ID:           "medicine_" + params.MedicineName,
			Enabled:      true,
			MedicineName: params.MedicineName,
			Times:        params.Times,
		}

		if err := t.store.SetReminder(reminder); err != nil {
			return "", fmt.Errorf("设置提醒失败: %w", err)
		}

		return fmt.Sprintf("已设置%s提醒，每天 %s 提醒你", params.MedicineName, strings.Join(params.Times, "、")), nil

	default:
		return "", fmt.Errorf("不支持的提醒类型: %s", params.ReminderType)
	}
}

// stopReminder 关闭提醒。
func (t *SetHealthReminderTool) stopReminder(params struct {
	ReminderType    string   `json:"reminder_type"`
	Action          string   `json:"action"`
	IntervalMinutes int      `json:"interval_minutes"`
	MedicineName    string   `json:"medicine_name"`
	Times           []string `json:"times"`
}) (string, error) {
	reminderType := ReminderType(params.ReminderType)

	var id string
	switch reminderType {
	case ReminderTypeWater:
		id = "water"
	case ReminderTypeExercise:
		id = "exercise"
	case ReminderTypeMedicine:
		if params.MedicineName == "" {
			return "", fmt.Errorf("关闭吃药提醒需要提供药品名称")
		}
		id = "medicine_" + params.MedicineName
	default:
		return "", fmt.Errorf("不支持的提醒类型: %s", params.ReminderType)
	}

	reminder := t.store.GetReminder(id)
	if reminder == nil {
		return fmt.Sprintf("%s提醒未开启", getReminderTypeName(reminderType)), nil
	}

	if err := t.store.DeleteReminder(id); err != nil {
		return "", fmt.Errorf("关闭提醒失败: %w", err)
	}

	return fmt.Sprintf("%s提醒已关闭", getReminderTypeName(reminderType)), nil
}

// getReminderTypeName 获取提醒类型中文名。
func getReminderTypeName(t ReminderType) string {
	switch t {
	case ReminderTypeWater:
		return "喝水"
	case ReminderTypeExercise:
		return "久坐"
	case ReminderTypeMedicine:
		return "吃药"
	default:
		return string(t)
	}
}

// ListHealthRemindersTool 列出健康提醒工具。
type ListHealthRemindersTool struct {
	store *HealthStore
}

// NewListHealthRemindersTool 创建列出健康提醒工具。
func NewListHealthRemindersTool(store *HealthStore) *ListHealthRemindersTool {
	return &ListHealthRemindersTool{store: store}
}

// Name 返回工具名称。
func (t *ListHealthRemindersTool) Name() string {
	return "list_health_reminders"
}

// Description 返回工具描述。
func (t *ListHealthRemindersTool) Description() string {
	return "列出所有已设置的健康提醒状态，包括喝水、久坐、吃药提醒的开启状态和配置信息。"
}

// Parameters 返回工具参数定义。
func (t *ListHealthRemindersTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type": "object", "properties": {}}`)
}

// Execute 执行工具。
func (t *ListHealthRemindersTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return t.store.FormatReminderStatus(), nil
}
