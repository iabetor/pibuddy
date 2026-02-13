package llm

import "context"

// Message 表示与 LLM 对话中的一条消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Provider 定义支持流式响应的 LLM 后端接口。
type Provider interface {
	// ChatStream 将对话消息发送给 LLM，返回一个 channel 逐块接收文本响应。
	ChatStream(ctx context.Context, messages []Message) (<-chan string, error)
}
