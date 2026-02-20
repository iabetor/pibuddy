package tools

import (
	"context"
	"encoding/json"
)

// GoToSleepTool 休息工具。
type GoToSleepTool struct{}

// NewGoToSleepTool 创建休息工具。
func NewGoToSleepTool() *GoToSleepTool {
	return &GoToSleepTool{}
}

func (t *GoToSleepTool) Name() string {
	return "go_to_sleep"
}

func (t *GoToSleepTool) Description() string {
	return "让助手进入休息状态，停止监听。当用户说'休息吧'、'不用了'、'去休息'、'停止监听'、'没事了'时调用此工具。"
}

func (t *GoToSleepTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

func (t *GoToSleepTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	// 返回特殊标记，Pipeline 会检测并停止监听
	return `{"success":true,"action":"sleep","message":"好的，我去休息了，有事再叫我"}`, nil
}
