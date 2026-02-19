package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSystemStatusTool(t *testing.T) {
	tool := NewSystemStatusTool()

	result, err := tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}

	if result == "" {
		t.Error("结果不应为空")
	}

	t.Logf("系统状态: %s", result)
}
