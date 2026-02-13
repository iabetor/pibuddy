package llm

import (
	"context"
	"encoding/json"
)

// Message 表示与 LLM 对话中的一条消息。
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall 表示 LLM 返回的一次工具调用。
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 表示函数调用的名称和参数。
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition 用于向 LLM 描述可用工具。
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition 描述工具的函数签名。
type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// StreamResult 是流式调用的结果，包含文本内容和可能的工具调用。
type StreamResult struct {
	Content   string
	ToolCalls []ToolCall
}

// Provider 定义支持流式响应的 LLM 后端接口。
type Provider interface {
	// ChatStream 将对话消息发送给 LLM，返回一个 channel 逐块接收文本响应。
	ChatStream(ctx context.Context, messages []Message) (<-chan string, error)

	// ChatStreamWithTools 将对话消息和工具定义发送给 LLM，
	// 返回一个 channel 逐块接收文本响应，以及可能的工具调用结果。
	ChatStreamWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (<-chan string, <-chan *StreamResult, error)
}
