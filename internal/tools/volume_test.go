package tools

import (
	"context"
	"encoding/json"
	"testing"
)

// mockVolumeController 模拟音量控制器用于测试
type mockVolumeController struct {
	volume int
	muted  bool
}

func (m *mockVolumeController) GetVolume() (int, error) {
	return m.volume, nil
}

func (m *mockVolumeController) SetVolume(volume int) error {
	m.volume = volume
	return nil
}

func (m *mockVolumeController) IsMuted() (bool, error) {
	return m.muted, nil
}

func (m *mockVolumeController) SetMute(muted bool) error {
	m.muted = muted
	return nil
}

func TestSetVolumeTool(t *testing.T) {
	tests := []struct {
		name     string
		initial  int
		args     setVolumeArgs
		expected int
	}{
		{"绝对设置", 50, setVolumeArgs{Volume: 80, Relative: false}, 80},
		{"相对增加", 80, setVolumeArgs{Volume: 10, Relative: true}, 90},
		{"相对减少", 80, setVolumeArgs{Volume: -10, Relative: true}, 70},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockVolumeController{volume: tt.initial, muted: false}
			tool := NewSetVolumeTool(mock, VolumeConfig{Step: 10})
			
			argsJSON, _ := json.Marshal(tt.args)
			_, err := tool.Execute(context.Background(), argsJSON)
			if err != nil {
				t.Fatalf("执行失败: %v", err)
			}
			if mock.volume != tt.expected {
				t.Errorf("期望音量 %d，实际 %d", tt.expected, mock.volume)
			}
		})
	}
}

func TestSetVolumeToolMute(t *testing.T) {
	mock := &mockVolumeController{volume: 50, muted: false}
	tool := NewSetVolumeTool(mock, VolumeConfig{Step: 10})

	// 静音
	args := setVolumeArgs{Volume: -1}
	argsJSON, _ := json.Marshal(args)
	result, err := tool.Execute(context.Background(), argsJSON)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if result != "已静音" {
		t.Errorf("期望 '已静音'，实际 '%s'", result)
	}

	// 取消静音
	argsJSON, _ = json.Marshal(args)
	result, err = tool.Execute(context.Background(), argsJSON)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if result != "已取消静音" {
		t.Errorf("期望 '已取消静音'，实际 '%s'", result)
	}
}

func TestGetVolumeTool(t *testing.T) {
	mock := &mockVolumeController{volume: 70, muted: false}
	tool := NewGetVolumeTool(mock)

	result, err := tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if result != "当前音量是70" {
		t.Errorf("期望 '当前音量是70'，实际 '%s'", result)
	}

	// 测试静音状态
	mock.muted = true
	result, err = tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if result != "当前已静音" {
		t.Errorf("期望 '当前已静音'，实际 '%s'", result)
	}
}

func TestVolumeBounds(t *testing.T) {
	mock := &mockVolumeController{volume: 50}
	tool := NewSetVolumeTool(mock, VolumeConfig{Step: 10})

	// 测试上限
	args := setVolumeArgs{Volume: 150, Relative: false}
	argsJSON, _ := json.Marshal(args)
	tool.Execute(context.Background(), argsJSON)
	if mock.volume != 100 {
		t.Errorf("期望音量 100，实际 %d", mock.volume)
	}

	// 测试下限
	args = setVolumeArgs{Volume: -50, Relative: false}
	argsJSON, _ = json.Marshal(args)
	tool.Execute(context.Background(), argsJSON)
	if mock.volume != 0 {
		t.Errorf("期望音量 0，实际 %d", mock.volume)
	}
}
