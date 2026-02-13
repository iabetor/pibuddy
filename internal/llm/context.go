package llm

import (
	"fmt"
	"time"
)

// ContextManager 使用滑动窗口维护对话历史，
// 在保持近期上下文的同时限制内存使用。
type ContextManager struct {
	systemPrompt string
	maxHistory   int
	messages     []Message
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
		cm.messages = cm.messages[len(cm.messages)-limit:]
	}
}

// Messages 返回发送给 LLM 的完整消息列表：系统消息 + 所有对话消息。
// system prompt 会动态追加当前日期时间，使 LLM 能理解"今天""明天"等相对时间。
func (cm *ContextManager) Messages() []Message {
	now := time.Now()
	weekdays := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	timeInfo := fmt.Sprintf("\n\n当前时间: %s %s %s",
		now.Format("2006年01月02日"),
		weekdays[now.Weekday()],
		now.Format("15:04"),
	)

	msgs := make([]Message, 0, 1+len(cm.messages))
	msgs = append(msgs, Message{
		Role:    "system",
		Content: cm.systemPrompt + timeInfo,
	})
	msgs = append(msgs, cm.messages...)
	return msgs
}

// Clear 清空对话历史。
func (cm *ContextManager) Clear() {
	cm.messages = cm.messages[:0]
}
