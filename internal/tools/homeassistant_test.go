package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHAListDevices(t *testing.T) {
	// 需要设置环境变量 PIBUDDY_HA_TOKEN
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiIwZmVhMzJiOWI4ZTA0YzYzOWI0Y2JiMGNhOWY0MTMwMSIsImlhdCI6MTc3MTUxMjgxOCwiZXhwIjoyMDg2ODcyODE4fQ.qpKZo4oQAZ0lXocSo3vtdS16WND1NWppeDdDUItgkd8"
	
	client := NewHomeAssistantClient("http://localhost:8123", token)
	tool := NewHAListDevicesTool(client)

	result, err := tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}

	if result == "" {
		t.Error("结果不应为空")
	}
	t.Logf("设备列表:\n%s", result)
}

func TestHAGetDeviceState(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiIwZmVhMzJiOWI4ZTA0YzYzOWI0Y2JiMGNhOWY0MTMwMSIsImlhdCI6MTc3MTUxMjgxOCwiZXhwIjoyMDg2ODcyODE4fQ.qpKZo4oQAZ0lXocSo3vtdS16WND1NWppeDdDUItgkd8"
	
	client := NewHomeAssistantClient("http://localhost:8123", token)
	tool := NewHAGetDeviceStateTool(client)

	// 测试获取小爱音箱状态
	args := haGetDeviceStateArgs{
		EntityID: "sensor.xiaomi_cn_320049009_l06a_playing_state_p_3_1",
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Execute(context.Background(), argsJSON)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}

	t.Logf("设备状态: %s", result)
}

func TestHAControlDevice(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiIwZmVhMzJiOWI4ZTA0YzYzOWI0Y2JiMGNhOWY0MTMwMSIsImlhdCI6MTc3MTUxMjgxOCwiZXhwIjoyMDg2ODcyODE4fQ.qpKZo4oQAZ0lXocSo3vtdS16WND1NWppeDdDUItgkd8"
	
	_ = NewHomeAssistantClient("http://localhost:8123", token)
	// 测试让小爱音箱播放文本（使用 notify 服务）
	// 注意：这需要通过 button 来触发，这里跳过实际控制测试
	t.Log("控制设备测试需要实际设备，跳过")
}
