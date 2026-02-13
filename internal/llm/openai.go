package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// sseChunk 表示 SSE 响应中的一个流式数据块。
type sseChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// ChatStream 向 OpenAI 兼容 API 发送对话消息，返回一个 channel 逐块接收文本响应。
func (p *OpenAIProvider) ChatStream(ctx context.Context, messages []Message) (<-chan string, error) {
	reqBody := chatRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   true,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("[llm] 序列化请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.apiURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("[llm] 创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[llm] 请求失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("[llm] API 返回状态码 %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan string)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				log.Println("[llm] 上下文已取消，停止读取 SSE")
				return
			default:
			}

			line := scanner.Text()

			// 跳过空行和非 data 行
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			// 流结束信号
			if data == "[DONE]" {
				log.Println("[llm] SSE 流结束")
				return
			}

			var chunk sseChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				log.Printf("[llm] 解析 SSE 数据块失败: %v", err)
				continue
			}

			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				if content != "" {
					select {
					case ch <- content:
					case <-ctx.Done():
						log.Println("[llm] 发送数据块时上下文已取消")
						return
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("[llm] 读取响应流出错: %v", err)
		}
	}()

	return ch, nil
}
