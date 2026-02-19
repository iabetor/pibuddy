package tools

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
)

// ReminderType 提醒类型。
type ReminderType string

const (
	ReminderTypeWater    ReminderType = "water"    // 喝水提醒
	ReminderTypeExercise ReminderType = "exercise" // 久坐提醒
	ReminderTypeMedicine ReminderType = "medicine" // 吃药提醒
)

// HealthReminder 健康提醒配置。
type HealthReminder struct {
	Type            ReminderType `json:"type"`
	ID              string       `json:"id"`              // 唯一标识
	Enabled         bool         `json:"enabled"`         // 是否启用
	IntervalMinutes int          `json:"interval_minutes"` // 间隔（分钟），用于 water/exercise
	LastRemindedAt  time.Time    `json:"last_reminded_at"` // 上次提醒时间
	MedicineName    string       `json:"medicine_name"`    // 药品名称，用于 medicine
	Times           []string     `json:"times"`           // 每日提醒时间，用于 medicine
	CreatedAt       time.Time    `json:"created_at"`
}

// TriggeredReminder 触发的提醒。
type TriggeredReminder struct {
	ID      string
	Message string
}

// HealthStoreConfig 存储配置。
type HealthStoreConfig struct {
	WaterInterval    int    // 默认喝水间隔（分钟）
	ExerciseInterval int    // 默认久坐间隔（分钟）
	QuietHoursStart  string // 静音开始时间，如 "23:00"
	QuietHoursEnd    string // 静音结束时间，如 "07:00"
}

// HealthStore 健康提醒存储。
type HealthStore struct {
	mu       sync.RWMutex
	filePath string
	config   HealthStoreConfig

	Reminders   map[string]*HealthReminder `json:"reminders"`
	QuietHours  QuietHours                 `json:"quiet_hours"`
	LastUpdated time.Time                  `json:"last_updated"`
}

// QuietHours 静音时段。
type QuietHours struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// NewHealthStore 创建健康提醒存储。
func NewHealthStore(dataDir string, config HealthStoreConfig) (*HealthStore, error) {
	store := &HealthStore{
		filePath:  filepath.Join(dataDir, "health_reminders.json"),
		config:    config,
		Reminders: make(map[string]*HealthReminder),
		QuietHours: QuietHours{
			Start: config.QuietHoursStart,
			End:   config.QuietHoursEnd,
		},
	}

	if err := store.load(); err != nil {
		logger.Warnf("[health] 加载存储失败: %v，使用默认配置", err)
	}

	return store, nil
}

// load 从文件加载配置。
func (s *HealthStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在是正常的
		}
		return fmt.Errorf("读取文件失败: %w", err)
	}

	if err := json.Unmarshal(data, s); err != nil {
		return fmt.Errorf("解析 JSON 失败: %w", err)
	}

	return nil
}

// save 保存配置到文件。
func (s *HealthStore) save() error {
	s.LastUpdated = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// IsQuietHours 检查当前是否在静音时段。
func (s *HealthStore) IsQuietHours() bool {
	now := time.Now()
	currentTime := now.Format("15:04")

	start := s.QuietHours.Start
	end := s.QuietHours.End

	if start == "" || end == "" {
		return false
	}

	// 跨天情况：如 23:00 - 07:00
	if start > end {
		return currentTime >= start || currentTime < end
	}
	// 同天情况：如 13:00 - 14:00
	return currentTime >= start && currentTime < end
}

// SetReminder 设置提醒。
func (s *HealthStore) SetReminder(reminder *HealthReminder) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if reminder.ID == "" {
		reminder.ID = string(reminder.Type)
		if reminder.Type == ReminderTypeMedicine {
			reminder.ID = "medicine_" + reminder.MedicineName
		}
	}

	reminder.CreatedAt = time.Now()
	s.Reminders[reminder.ID] = reminder

	return s.save()
}

// GetReminder 获取提醒。
func (s *HealthStore) GetReminder(id string) *HealthReminder {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Reminders[id]
}

// ListReminders 列出所有提醒。
func (s *HealthStore) ListReminders() []*HealthReminder {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reminders := make([]*HealthReminder, 0, len(s.Reminders))
	for _, r := range s.Reminders {
		reminders = append(reminders, r)
	}

	return reminders
}

// DeleteReminder 删除提醒。
func (s *HealthStore) DeleteReminder(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Reminders, id)

	return s.save()
}

// CheckAndTrigger 检查并触发到期的提醒。
func (s *HealthStore) CheckAndTrigger() []*TriggeredReminder {
	s.mu.Lock()
	defer s.mu.Unlock()

	var triggered []*TriggeredReminder
	now := time.Now()
	isQuiet := s.IsQuietHours()

	for id, reminder := range s.Reminders {
		if !reminder.Enabled {
			continue
		}

		var shouldTrigger bool
		var message string

		switch reminder.Type {
		case ReminderTypeWater, ReminderTypeExercise:
			// 间隔型提醒
			elapsed := now.Sub(reminder.LastRemindedAt)
			if elapsed >= time.Duration(reminder.IntervalMinutes)*time.Minute {
				shouldTrigger = true
				message = s.getRandomMessage(reminder.Type)
			}

		case ReminderTypeMedicine:
			// 时间点提醒
			currentTime := now.Format("15:04")
			currentDate := now.Format("2006-01-02")
			for _, t := range reminder.Times {
				if currentTime == t {
					// 检查今天这个时间点是否已提醒
					lastRemindedDate := reminder.LastRemindedAt.Format("2006-01-02")
					lastRemindedTime := reminder.LastRemindedAt.Format("15:04")
					if lastRemindedDate != currentDate || lastRemindedTime != t {
						shouldTrigger = true
						message = fmt.Sprintf("该吃%s了，别忘了", reminder.MedicineName)
						break
					}
				}
			}
		}

		if shouldTrigger {
			reminder.LastRemindedAt = now
			triggered = append(triggered, &TriggeredReminder{
				ID:      id,
				Message: message,
			})
		}
	}

	if len(triggered) > 0 {
		if err := s.save(); err != nil {
			logger.Warnf("[health] 保存状态失败: %v", err)
		}
	}

	// 静音时段不返回提醒
	if isQuiet {
		return nil
	}

	return triggered
}

// getRandomMessage 获取随机提醒文案。
func (s *HealthStore) getRandomMessage(reminderType ReminderType) string {
	var messages []string

	switch reminderType {
	case ReminderTypeWater:
		messages = []string{
			"该喝水了，身体需要水分补充",
			"喝杯水吧，保持健康好习惯",
			"记得喝水哦，多喝水对身体好",
			"休息一下，喝杯水吧",
			"水是生命之源，该补充水分了",
		}
	case ReminderTypeExercise:
		messages = []string{
			"坐太久了，起来活动一下吧",
			"该起来走走了，久坐对身体不好",
			"站起来活动一下，放松放松",
			"休息一下，伸个懒腰吧",
			"久坐伤身，起来动一动",
		}
	default:
		return "该休息一下了"
	}

	return messages[rand.Intn(len(messages))]
}

// FormatReminderStatus 格式化提醒状态。
func (s *HealthStore) FormatReminderStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.Reminders) == 0 {
		return "当前没有设置任何健康提醒"
	}

	var result string
	for _, r := range s.Reminders {
		status := "关闭"
		if r.Enabled {
			status = "开启"
		}

		switch r.Type {
		case ReminderTypeWater:
			result += fmt.Sprintf("- 喝水提醒：%s（每%d分钟）\n", status, r.IntervalMinutes)
		case ReminderTypeExercise:
			result += fmt.Sprintf("- 久坐提醒：%s（每%d分钟）\n", status, r.IntervalMinutes)
		case ReminderTypeMedicine:
			times := ""
			for i, t := range r.Times {
				if i > 0 {
					times += "、"
				}
				times += t
			}
			result += fmt.Sprintf("- %s提醒：%s（%s）\n", r.MedicineName, status, times)
		}
	}

	result += fmt.Sprintf("\n静音时段：%s - %s", s.QuietHours.Start, s.QuietHours.End)

	return result
}
