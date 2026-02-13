package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// DateTimeTool 返回当前日期时间信息。
type DateTimeTool struct{}

func NewDateTimeTool() *DateTimeTool {
	return &DateTimeTool{}
}

func (t *DateTimeTool) Name() string { return "get_datetime" }

func (t *DateTimeTool) Description() string {
	return "获取当前的日期、时间和星期几。当用户询问现在几点、今天几号、今天星期几等问题时使用。"
}

func (t *DateTimeTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}

func (t *DateTimeTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	now := time.Now()

	weekdays := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	weekday := weekdays[now.Weekday()]

	return fmt.Sprintf("当前时间: %s %s %s",
		now.Format("2006年01月02日"),
		weekday,
		now.Format("15:04:05"),
	), nil
}
