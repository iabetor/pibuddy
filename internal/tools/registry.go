package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/iabetor/pibuddy/internal/llm"
)

// Tool 定义工具接口，每个工具必须自描述。
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry 管理所有已注册工具。
type Registry struct {
	tools map[string]Tool
}

// NewRegistry 创建工具注册表。
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register 注册一个工具。
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
	log.Printf("[tools] 已注册工具: %s", t.Name())
}

// Get 获取指定名称的工具。
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Definitions 返回所有工具的定义，用于发送给 LLM。
func (r *Registry) Definitions() []llm.ToolDefinition {
	defs := make([]llm.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionDefinition{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return defs
}

// Execute 执行指定工具并返回结果。
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("未知工具: %s", name)
	}
	log.Printf("[tools] 执行工具: %s, 参数: %s", name, string(args))
	result, err := t.Execute(ctx, args)
	if err != nil {
		log.Printf("[tools] 工具 %s 执行失败: %v", name, err)
		return "", err
	}
	log.Printf("[tools] 工具 %s 执行成功", name)
	return result, nil
}

// Count 返回已注册工具数量。
func (r *Registry) Count() int {
	return len(r.tools)
}
