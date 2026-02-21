package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"github.com/iabetor/pibuddy/internal/logger"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider 通过 SSE（Server-Sent Events）与 OpenAI 兼容的 API 通信，
// 支持流式接收大模型回复。
type OpenAIProvider struct {
	apiURL     string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAIProvider 创建一个新的 OpenAI 兼容 LLM 提供者。
func NewOpenAIProvider(apiURL, apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		apiURL: apiURL,
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// chatRequest 是发送到 chat completions 接口的 JSON 请求体。
type chatRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Stream   bool             `json:"stream"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

// sseChunk 表示 SSE 响应中的一个流式数据块。
type sseChunk struct {
	Choices []sseChoice `json:"choices"`
}

type sseChoice struct {
	Delta        sseDelta `json:"delta"`
	FinishReason *string  `json:"finish_reason"`
}

type sseDelta struct {
	Content   string        `json:"content"`
	ToolCalls []sseToolCall `json:"tool_calls,omitempty"`
}

type sseToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

// ChatStream 向 OpenAI 兼容 API 发送对话消息，返回一个 channel 逐块接收文本响应。
func (p *OpenAIProvider) ChatStream(ctx context.Context, messages []Message) (<-chan string, error) {
	textCh, resultCh, err := p.ChatStreamWithTools(ctx, messages, nil)
	if err != nil {
		return nil, err
	}
	// 丢弃 resultCh
	go func() {
		for range resultCh {
		}
	}()
	return textCh, nil
}

// ChatStreamWithTools 向 OpenAI 兼容 API 发送带工具定义的对话消息。
// textCh 逐块返回文本内容，resultCh 在流结束时返回最终结果（包含可能的 tool_calls）。
func (p *OpenAIProvider) ChatStreamWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (<-chan string, <-chan *StreamResult, error) {
	reqBody := chatRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   true,
		Tools:    tools,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("[llm] 序列化请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.apiURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("[llm] 创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("[llm] 请求失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodyStr := string(body)
		bodyLower := strings.ToLower(bodyStr)
		// 检查是否为余额不足/额度耗尽错误
		// DeepSeek: HTTP 402 + "Insufficient Balance"
		// 千问 DashScope: HTTP 429 + quota 相关
		// 火山方舟: HTTP 429 + rate limit / quota
		if resp.StatusCode == 402 ||
			(resp.StatusCode == 429 && (strings.Contains(bodyLower, "quota") || strings.Contains(bodyLower, "insufficient"))) {
			return nil, nil, fmt.Errorf("[llm] API 返回状态码 %d: %s: %w", resp.StatusCode, bodyStr, ErrInsufficientBalance)
		}
		return nil, nil, fmt.Errorf("[llm] API 返回状态码 %d: %s", resp.StatusCode, bodyStr)
	}

	textCh := make(chan string)
	resultCh := make(chan *StreamResult, 1)

	go func() {
		defer close(textCh)
		defer close(resultCh)
		defer resp.Body.Close()

		var contentBuf strings.Builder
		// toolCallsMap 用于增量拼接流式返回的 tool_calls
		toolCallsMap := make(map[int]*ToolCall)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				logger.Debug("[llm] 上下文已取消，停止读取 SSE")
				return
			default:
			}

			line := scanner.Text()

			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			if data == "[DONE]" {
				logger.Debug("[llm] SSE 流结束")
				break
			}

			var chunk sseChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				logger.Warnf("[llm] 解析 SSE 数据块失败: %v", err)
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			choice := chunk.Choices[0]
			delta := choice.Delta

			// 处理文本内容
			if delta.Content != "" {
				contentBuf.WriteString(delta.Content)
				select {
				case textCh <- delta.Content:
				case <-ctx.Done():
					logger.Debug("[llm] 发送数据块时上下文已取消")
					return
				}
			}

			// 处理 tool_calls 增量拼接
			for _, tc := range delta.ToolCalls {
				existing, ok := toolCallsMap[tc.Index]
				if !ok {
					toolCallsMap[tc.Index] = &ToolCall{
						ID:   tc.ID,
						Type: tc.Type,
						Function: FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				} else {
					if tc.ID != "" {
						existing.ID = tc.ID
					}
					if tc.Type != "" {
						existing.Type = tc.Type
					}
					if tc.Function.Name != "" {
						existing.Function.Name += tc.Function.Name
					}
					existing.Function.Arguments += tc.Function.Arguments
				}
			}
		}

		if err := scanner.Err(); err != nil {
			logger.Errorf("[llm] 读取响应流出错: %v", err)
		}

		// 构建最终结果
		result := &StreamResult{
			Content: contentBuf.String(),
		}
		if len(toolCallsMap) > 0 {
			calls := make([]ToolCall, 0, len(toolCallsMap))
			for i := 0; i < len(toolCallsMap); i++ {
				if tc, ok := toolCallsMap[i]; ok {
					calls = append(calls, *tc)
				}
			}
			result.ToolCalls = calls
			logger.Infof("[llm] 检测到 %d 个工具调用", len(calls))
		}
		resultCh <- result
	}()

	return textCh, resultCh, nil
}
