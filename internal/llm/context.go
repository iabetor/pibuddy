package llm

import (
	"fmt"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
)

// UserPreferences 用户偏好接口，由 voiceprint 包提供实现。
type UserPreferences interface {
	GetPreferences() string // 返回 JSON 格式的偏好
	IsOwner() bool
}

// ContextManager 使用滑动窗口维护对话历史，
// 在保持近期上下文的同时限制内存使用。
type ContextManager struct {
	systemPrompt   string
	maxHistory     int
	messages       []Message
	currentSpeaker string
	speakerInfo    UserPreferences // 当前说话人信息
}

// NewContextManager 创建对话上下文管理器。
// systemPrompt: 系统提示词
// maxHistory: 最多保留的对话轮数（一问一答算两条消息）
func NewContextManager(systemPrompt string, maxHistory int) *ContextManager {
	return &ContextManager{
		systemPrompt: systemPrompt,
		maxHistory:   maxHistory,
		messages:     make([]Message, 0),
	}
}

// SetCurrentSpeaker 设置当前说话人。info 可为 nil 表示未识别。
func (cm *ContextManager) SetCurrentSpeaker(name string, info UserPreferences) {
	cm.currentSpeaker = name
	cm.speakerInfo = info
}

// GetCurrentSpeaker 获取当前说话人姓名。
func (cm *ContextManager) GetCurrentSpeaker() string {
	return cm.currentSpeaker
}

// Add 添加一条消息到对话历史。
// 当消息数超过 maxHistory*2 时，自动截掉最早的消息只保留最近的部分。
func (cm *ContextManager) Add(role, content string) {
	cm.AddMessage(Message{
		Role:    role,
		Content: content,
	})
}

// AddMessage 添加一条完整消息到对话历史（支持 tool_calls 等字段）。
func (cm *ContextManager) AddMessage(msg Message) {
	cm.messages = append(cm.messages, msg)

	limit := cm.maxHistory * 2
	if len(cm.messages) > limit {
		// 截断后需要清理孤立的 tool 消息（没有对应的 tool_calls）
		cm.messages = cm.messages[len(cm.messages)-limit:]
		cm.cleanupOrphanToolMessages()
	}
}

// cleanupOrphanToolMessages 清理没有对应 tool_calls 的 tool 消息。
func (cm *ContextManager) cleanupOrphanToolMessages() {
	// 收集所有 tool_call_id
	toolCallIDs := make(map[string]bool)
	for _, msg := range cm.messages {
		for _, tc := range msg.ToolCalls {
			toolCallIDs[tc.ID] = true
		}
	}

	// 过滤掉孤立的 tool 消息
	var cleaned []Message
	for _, msg := range cm.messages {
		if msg.Role == "tool" {
			// 只保留有对应 tool_call 的消息
			if toolCallIDs[msg.ToolCallID] {
				cleaned = append(cleaned, msg)
			}
		} else {
			cleaned = append(cleaned, msg)
		}
	}
	cm.messages = cleaned
}

// Messages 返回发送给 LLM 的完整消息列表：系统消息 + 所有对话消息。
// system prompt 会动态追加当前日期时间和用户偏好，使 LLM 能理解"今天""明天"等相对时间，
// 并根据用户偏好调整回复风格。
func (cm *ContextManager) Messages() []Message {
	now := time.Now()
	weekdays := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	timeInfo := fmt.Sprintf("\n\n当前时间: %s %s %s",
		now.Format("2006年01月02日"),
		weekdays[now.Weekday()],
		now.Format("15:04"),
	)

	// 构建用户信息
	var userInfo string
	if cm.currentSpeaker != "" {
		userInfo = fmt.Sprintf("\n当前对话用户: %s", cm.currentSpeaker)
		if cm.speakerInfo != nil && cm.speakerInfo.GetPreferences() != "" {
			userInfo += fmt.Sprintf("\n用户偏好: %s", cm.speakerInfo.GetPreferences())
		}
	}

	// 清理消息序列，确保格式正确
	messages := cm.cleanMessageSequence(cm.messages)

	msgs := make([]Message, 0, 1+len(messages))
	msgs = append(msgs, Message{
		Role:    "system",
		Content: cm.systemPrompt + timeInfo + userInfo,
	})
	msgs = append(msgs, messages...)
	return msgs
}

// cleanMessageSequence 清理消息序列。
// 正常的工具调用流程中，消息会以 tool 结尾（assistant(tool_calls) + tool(result)），
// 这是正确的序列，LLM 需要看到 tool 结果才能生成回复，必须保留！
// 只有孤立的 tool 消息（没有对应的 assistant(tool_calls)）才需要清理。
func (cm *ContextManager) cleanMessageSequence(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}

	// 末尾不是 tool 消息，无需处理
	if messages[len(messages)-1].Role != "tool" {
		return messages
	}

	// 末尾是 tool 消息，检查是否有对应的 assistant(tool_calls)
	// 向前找到所有连续的 tool 消息
	toolStart := len(messages) - 1
	for toolStart > 0 && messages[toolStart-1].Role == "tool" {
		toolStart--
	}

	// 检查 tool 消息前面是否是 assistant(tool_calls)
	if toolStart > 0 {
		prev := messages[toolStart-1]
		if prev.Role == "assistant" && len(prev.ToolCalls) > 0 {
			// 这是完整的工具调用序列：assistant(tool_calls) + tool(result)
			// 保留，LLM 需要看到工具结果来生成回复
			return messages
		}
	}

	// 孤立的 tool 消息（没有对应的 assistant tool_calls），清理掉
	cleaned := make([]Message, len(messages))
	copy(cleaned, messages)

	logger.Debugf("[context] cleanMessageSequence: 清理孤立 tool 消息")

	for len(cleaned) > 0 && cleaned[len(cleaned)-1].Role == "tool" {
		cleaned = cleaned[:len(cleaned)-1]
	}

	logger.Debugf("[context] cleanMessageSequence: %d -> %d 条消息", len(messages), len(cleaned))
	return cleaned
}

// RemoveLastMessages 移除最后 n 条消息。
// 用于在工具调用后直接返回时清理不完整的消息序列。
func (cm *ContextManager) RemoveLastMessages(n int) {
	if n <= 0 || n > len(cm.messages) {
		return
	}
	cm.messages = cm.messages[:len(cm.messages)-n]
}

// Clear 清空对话历史。
func (cm *ContextManager) Clear() {
	cm.messages = cm.messages[:0]
}
