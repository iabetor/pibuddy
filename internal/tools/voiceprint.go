package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/iabetor/pibuddy/internal/audio"
	"github.com/iabetor/pibuddy/internal/logger"
	"github.com/iabetor/pibuddy/internal/voiceprint"
)

// VoiceprintConfig 声纹工具配置。
type VoiceprintConfig struct {
	Manager     *voiceprint.Manager
	Capture     *audio.Capture
	SampleRate  int
	OwnerName   string // 主人姓名
}

// registerVoiceprintResult 注册声纹结果。
type registerVoiceprintResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// deleteVoiceprintResult 删除声纹结果。
type deleteVoiceprintResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// toJSON 将任意值转换为 JSON 字符串。
func toJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"message":"JSON序列化失败: %v"}`, err)
	}
	return string(b)
}

// RegisterVoiceprintTool 注册声纹工具。
type RegisterVoiceprintTool struct {
	cfg VoiceprintConfig
}

// NewRegisterVoiceprintTool 创建注册声纹工具。
func NewRegisterVoiceprintTool(cfg VoiceprintConfig) *RegisterVoiceprintTool {
	return &RegisterVoiceprintTool{cfg: cfg}
}

func (t *RegisterVoiceprintTool) Name() string {
	return "register_voiceprint"
}

func (t *RegisterVoiceprintTool) Description() string {
	return "注册新用户的声纹。只有主人可以使用此功能。参数: name(用户名), preferences(可选，用户偏好JSON)"
}

func (t *RegisterVoiceprintTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "要注册的用户名"
			},
			"preferences": {
				"type": "string",
				"description": "用户偏好JSON，如 {\"style\":\"简洁直接\",\"interests\":[\"编程\"]}"
			}
		},
		"required": ["name"]
	}`)
}

// Execute 执行注册声纹。
// 注意：此工具需要用户配合说话，会阻塞一段时间。
func (t *RegisterVoiceprintTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Name        string `json:"name"`
		Preferences string `json:"preferences"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	// 检查是否有声纹管理器
	if t.cfg.Manager == nil {
		return toJSON(registerVoiceprintResult{
			Success: false,
			Message: "声纹识别未启用",
		}), nil
	}

	// 录制声纹样本
	const numSamples = 3
	const sampleDuration = 3 * time.Second

	var samples [][]float32
	for i := 0; i < numSamples; i++ {
		logger.Infof("[voiceprint-tool] 录制第 %d/%d 个样本...", i+1, numSamples)

		recordCtx, cancel := context.WithTimeout(ctx, sampleDuration)
		recorded := t.cfg.Capture.RecordFor(recordCtx)
		cancel()

		if len(recorded) < t.cfg.SampleRate {
			logger.Warnf("[voiceprint-tool] 录制数据不足，跳过")
			continue
		}
		samples = append(samples, recorded)
	}

	if len(samples) < 2 {
		return toJSON(registerVoiceprintResult{
			Success: false,
			Message: "录制样本不足，请重新尝试",
		}), nil
	}

	// 注册用户
	if err := t.cfg.Manager.Register(params.Name, samples); err != nil {
		return toJSON(registerVoiceprintResult{
			Success: false,
			Message: fmt.Sprintf("注册失败: %v", err),
		}), nil
	}

	// 设置偏好（如果有）
	if params.Preferences != "" {
		if err := t.cfg.Manager.SetPreferences(params.Name, params.Preferences); err != nil {
			logger.Warnf("[voiceprint-tool] 设置偏好失败: %v", err)
		}
	}

	// 如果是第一个用户，自动设为主人
	if t.cfg.Manager.NumSpeakers() == 1 || params.Name == t.cfg.OwnerName {
		if err := t.cfg.Manager.SetOwner(params.Name); err != nil {
			logger.Warnf("[voiceprint-tool] 设置主人失败: %v", err)
		}
	}

	return toJSON(registerVoiceprintResult{
		Success: true,
		Message: fmt.Sprintf("用户 %s 注册成功", params.Name),
	}), nil
}

// DeleteVoiceprintTool 删除声纹工具。
type DeleteVoiceprintTool struct {
	cfg VoiceprintConfig
}

// NewDeleteVoiceprintTool 创建删除声纹工具。
func NewDeleteVoiceprintTool(cfg VoiceprintConfig) *DeleteVoiceprintTool {
	return &DeleteVoiceprintTool{cfg: cfg}
}

func (t *DeleteVoiceprintTool) Name() string {
	return "delete_voiceprint"
}

func (t *DeleteVoiceprintTool) Description() string {
	return "删除用户的声纹。只有主人可以使用此功能。参数: name(要删除的用户名)"
}

func (t *DeleteVoiceprintTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "要删除的用户名"
			}
		},
		"required": ["name"]
	}`)
}

// Execute 执行删除声纹。
func (t *DeleteVoiceprintTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	// 检查是否有声纹管理器
	if t.cfg.Manager == nil {
		return toJSON(deleteVoiceprintResult{
			Success: false,
			Message: "声纹识别未启用",
		}), nil
	}

	// 删除用户
	if err := t.cfg.Manager.DeleteUser(params.Name); err != nil {
		return toJSON(deleteVoiceprintResult{
			Success: false,
			Message: fmt.Sprintf("删除失败: %v", err),
		}), nil
	}

	return toJSON(deleteVoiceprintResult{
		Success: true,
		Message: fmt.Sprintf("用户 %s 已删除", params.Name),
	}), nil
}

// SetPreferencesTool 设置用户偏好工具。
type SetPreferencesTool struct {
	cfg VoiceprintConfig
}

// NewSetPreferencesTool 创建设置偏好工具。
func NewSetPreferencesTool(cfg VoiceprintConfig) *SetPreferencesTool {
	return &SetPreferencesTool{cfg: cfg}
}

func (t *SetPreferencesTool) Name() string {
	return "set_user_preferences"
}

func (t *SetPreferencesTool) Description() string {
	return "设置用户的回复风格偏好。只有主人可以设置。参数: name(用户名), preferences(偏好JSON)"
}

func (t *SetPreferencesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "用户名"
			},
			"preferences": {
				"type": "string",
				"description": "用户偏好JSON，如 {\"style\":\"简洁直接\",\"interests\":[\"编程\"],\"nickname\":\"程序员\"}"
			}
		},
		"required": ["name", "preferences"]
	}`)
}

// Execute 执行设置偏好。
func (t *SetPreferencesTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Name        string `json:"name"`
		Preferences string `json:"preferences"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	// 检查是否有声纹管理器
	if t.cfg.Manager == nil {
		return `{"success":false,"message":"声纹识别未启用"}`, nil
	}

	// 验证 JSON 格式
	var prefs voiceprint.UserPreferences
	if err := json.Unmarshal([]byte(params.Preferences), &prefs); err != nil {
		return `{"success":false,"message":"偏好JSON格式错误"}`, nil
	}

	// 设置偏好
	if err := t.cfg.Manager.SetPreferences(params.Name, params.Preferences); err != nil {
		return fmt.Sprintf(`{"success":false,"message":"设置失败: %v"}`, err), nil
	}

	return fmt.Sprintf(`{"success":true,"message":"已为 %s 设置偏好"}`, params.Name), nil
}
