package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/iabetor/pibuddy/internal/logger"
)

// ModelConfig 描述一个 LLM 模型的连接信息。
type ModelConfig struct {
	Name   string // 显示名称
	APIURL string // API 地址
	APIKey string // API Key
	Model  string // 模型名称或接入点 ID
}

// providerEntry 是一个 Provider 及其配置的组合。
type providerEntry struct {
	name     string
	provider Provider
}

// MultiProvider 实现多 LLM 自动降级。
// 按优先级列表顺序尝试，当前模型请求失败时自动切换到下一个。
type MultiProvider struct {
	entries []providerEntry
	current int // 当前活跃索引
	mu      sync.RWMutex
}

// NewMultiProvider 根据模型配置列表创建 MultiProvider。
func NewMultiProvider(configs []ModelConfig) (*MultiProvider, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("至少需要一个 LLM 模型配置")
	}

	entries := make([]providerEntry, 0, len(configs))
	for _, cfg := range configs {
		p := NewOpenAIProvider(cfg.APIURL, cfg.APIKey, cfg.Model)
		entries = append(entries, providerEntry{
			name:     cfg.Name,
			provider: p,
		})
	}

	logger.Infof("[llm] 多模型已初始化，共 %d 个模型：%s",
		len(entries), formatModelNames(entries))

	return &MultiProvider{
		entries: entries,
		current: 0,
	}, nil
}

// CurrentName 返回当前活跃模型的名称。
func (m *MultiProvider) CurrentName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.entries[m.current].name
}

// ChatStream 实现 Provider 接口，自动降级。
func (m *MultiProvider) ChatStream(ctx context.Context, messages []Message) (<-chan string, error) {
	textCh, resultCh, err := m.ChatStreamWithTools(ctx, messages, nil)
	if err != nil {
		return nil, err
	}
	go func() {
		for range resultCh {
		}
	}()
	return textCh, nil
}

// ChatStreamWithTools 实现 Provider 接口，支持自动降级。
// 从当前活跃模型开始尝试，失败时切换到下一个，直到所有模型都尝试过。
func (m *MultiProvider) ChatStreamWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (<-chan string, <-chan *StreamResult, error) {
	m.mu.RLock()
	startIdx := m.current
	total := len(m.entries)
	m.mu.RUnlock()

	var lastErr error

	for i := 0; i < total; i++ {
		idx := (startIdx + i) % total
		entry := m.entries[idx]

		logger.Debugf("[llm] 尝试模型 [%s] (索引 %d/%d)", entry.name, idx+1, total)

		textCh, resultCh, err := entry.provider.ChatStreamWithTools(ctx, messages, tools)
		if err == nil {
			// 成功，更新当前索引
			if idx != startIdx {
				m.mu.Lock()
				m.current = idx
				m.mu.Unlock()
				logger.Infof("[llm] 切换到模型 [%s]", entry.name)
			}
			return textCh, resultCh, nil
		}

		lastErr = err
		logger.Warnf("[llm] 模型 [%s] 请求失败: %v", entry.name, err)

		// 判断是否应该降级（额度耗尽、速率限制、服务不可用）
		if shouldFallback(err) {
			logger.Infof("[llm] 模型 [%s] 触发降级，尝试下一个模型", entry.name)
			// 更新当前索引到下一个，避免下次请求还走这个失败的
			nextIdx := (idx + 1) % total
			m.mu.Lock()
			m.current = nextIdx
			m.mu.Unlock()
			continue
		}

		// 非降级类错误（如上下文取消），直接返回
		return nil, nil, err
	}

	return nil, nil, fmt.Errorf("所有 LLM 模型均不可用，最后错误: %w", lastErr)
}

// shouldFallback 判断错误是否应该触发降级到下一个模型。
func shouldFallback(err error) bool {
	if err == nil {
		return false
	}

	// 余额不足
	if IsInsufficientBalance(err) {
		return true
	}

	errMsg := strings.ToLower(err.Error())

	// HTTP 状态码类错误
	if strings.Contains(errMsg, "状态码 402") ||
		strings.Contains(errMsg, "状态码 429") ||
		strings.Contains(errMsg, "状态码 503") ||
		strings.Contains(errMsg, "status code 402") ||
		strings.Contains(errMsg, "status code 429") ||
		strings.Contains(errMsg, "status code 503") {
		return true
	}

	// 关键词匹配
	fallbackKeywords := []string{
		"insufficient", "balance", "quota",
		"rate limit", "too many requests",
		"余额不足", "额度", "限流",
	}
	for _, kw := range fallbackKeywords {
		if strings.Contains(errMsg, kw) {
			return true
		}
	}

	// 网络/超时类错误
	if strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "deadline exceeded") ||
		strings.Contains(errMsg, "connection refused") {
		return true
	}

	return false
}

// formatModelNames 格式化模型名称列表用于日志。
func formatModelNames(entries []providerEntry) string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	return strings.Join(names, " → ")
}
