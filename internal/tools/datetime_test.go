package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDateTimeTool_Name(t *testing.T) {
	tool := NewDateTimeTool()
	if tool.Name() != "get_datetime" {
		t.Errorf("expected name 'get_datetime', got %q", tool.Name())
	}
}

func TestDateTimeTool_Execute(t *testing.T) {
	tool := NewDateTimeTool()
	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "当前时间") {
		t.Errorf("result should contain '当前时间', got %q", result)
	}

	now := time.Now()
	dateStr := now.Format("2006年01月02日")
	if !strings.Contains(result, dateStr) {
		t.Errorf("result should contain today's date %q, got %q", dateStr, result)
	}

	weekdays := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	weekday := weekdays[now.Weekday()]
	if !strings.Contains(result, weekday) {
		t.Errorf("result should contain weekday %q, got %q", weekday, result)
	}
}
