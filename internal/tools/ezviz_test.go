package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func getEzvizClient(t *testing.T) *EzvizClient {
	ak := os.Getenv("PIBUDDY_EZVIZ_AK")
	sk := os.Getenv("PIBUDDY_EZVIZ_SK")
	if ak == "" || sk == "" {
		t.Skip("PIBUDDY_EZVIZ_AK 和 PIBUDDY_EZVIZ_SK 未设置，跳过集成测试")
	}
	return NewEzvizClient(ak, sk)
}

func TestEzvizGetAccessToken(t *testing.T) {
	client := getEzvizClient(t)
	token, err := client.getAccessToken()
	if err != nil {
		t.Fatalf("获取 accessToken 失败: %v", err)
	}
	if token == "" {
		t.Fatal("accessToken 为空")
	}
	t.Logf("accessToken: %s...", token[:16])

	// 二次调用应使用缓存
	token2, err := client.getAccessToken()
	if err != nil {
		t.Fatalf("二次获取失败: %v", err)
	}
	if token != token2 {
		t.Fatal("token 缓存未生效")
	}
}

func TestEzvizListDevices(t *testing.T) {
	client := getEzvizClient(t)
	devices, err := client.ListDevices()
	if err != nil {
		t.Fatalf("获取设备列表失败: %v", err)
	}
	t.Logf("设备数量: %d", len(devices))
	for _, d := range devices {
		t.Logf("  - %s (%s) [%s] 状态:%d", d.DeviceName, d.ParentCategory, d.DeviceSerial, d.Status)
	}
}

func TestEzvizGetDeviceInfo(t *testing.T) {
	client := getEzvizClient(t)
	serial := os.Getenv("PIBUDDY_EZVIZ_DEVICE_SERIAL")
	if serial == "" {
		serial = "BC6385600"
	}

	info, err := client.GetDeviceInfo(serial)
	if err != nil {
		t.Fatalf("获取设备信息失败: %v", err)
	}
	t.Logf("设备: %s, 型号: %s, 类型: %s, 状态: %d, 信号: %s",
		info.DeviceName, info.Model, info.ParentCategory, info.Status, info.Signal)
}

func TestEzvizGetDeviceCapacity(t *testing.T) {
	client := getEzvizClient(t)
	serial := os.Getenv("PIBUDDY_EZVIZ_DEVICE_SERIAL")
	if serial == "" {
		serial = "BC6385600"
	}

	cap, err := client.GetDeviceCapacity(serial)
	if err != nil {
		t.Fatalf("获取设备能力失败: %v", err)
	}
	t.Logf("远程开门: %s, 验证码认证: %s, 门状态检查: %s, 电池百分比: %s",
		cap.SupportRemoteOpenDoor, cap.SupportRemoteAuthRandcode,
		cap.SupportCheckDoorState, cap.SupportLockBatteryPerCent)
}

func TestEzvizListDevicesTool(t *testing.T) {
	client := getEzvizClient(t)
	tool := NewEzvizListDevicesTool(client)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	t.Logf("结果:\n%s", result)
}

func TestEzvizLockStatusTool(t *testing.T) {
	client := getEzvizClient(t)
	tool := NewEzvizGetLockStatusTool(client, "BC6385600")

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	t.Logf("结果:\n%s", result)
}
