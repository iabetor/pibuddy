package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
)

// HomeAssistantConfig Home Assistant 配置。
type HomeAssistantConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Token   string `yaml:"token"`
}

// HomeAssistantClient Home Assistant API 客户端。
type HomeAssistantClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewHomeAssistantClient 创建 Home Assistant 客户端。
func NewHomeAssistantClient(url, token string) *HomeAssistantClient {
	return &HomeAssistantClient{
		baseURL: strings.TrimSuffix(url, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DeviceState 设备状态。
type DeviceState struct {
	EntityID    string                 `json:"entity_id"`
	State       string                 `json:"state"`
	Attributes  map[string]interface{} `json:"attributes"`
	LastChanged string                 `json:"last_changed"`
}

// doRequest 执行 HTTP 请求。
func (c *HomeAssistantClient) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API 错误 (状态码 %d): %s", resp.StatusCode, string(data))
	}

	return data, nil
}

// GetStates 获取所有设备状态。
func (c *HomeAssistantClient) GetStates() ([]DeviceState, error) {
	data, err := c.doRequest("GET", "/api/states", nil)
	if err != nil {
		return nil, err
	}

	var states []DeviceState
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return states, nil
}

// GetState 获取单个设备状态。
func (c *HomeAssistantClient) GetState(entityID string) (*DeviceState, error) {
	data, err := c.doRequest("GET", "/api/states/"+entityID, nil)
	if err != nil {
		return nil, err
	}

	var state DeviceState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &state, nil
}

// CallService 调用服务。
func (c *HomeAssistantClient) CallService(domain, service string, data map[string]interface{}) error {
	_, err := c.doRequest("POST", fmt.Sprintf("/api/services/%s/%s", domain, service), data)
	return err
}

// --- 工具定义 ---

// HAListDevicesTool 列出设备工具。
type HAListDevicesTool struct {
	client *HomeAssistantClient
}

// NewHAListDevicesTool 创建列出设备工具。
func NewHAListDevicesTool(client *HomeAssistantClient) *HAListDevicesTool {
	return &HAListDevicesTool{client: client}
}

func (t *HAListDevicesTool) Name() string {
	return "ha_list_devices"
}

func (t *HAListDevicesTool) Description() string {
	return "列出所有可控制的智能家居设备。**控制或查询设备前必须先调用此工具获取正确的 entity_id**。可按类型筛选：light(灯)、switch(开关)、fan(风扇)、climate(空调)、cover(窗帘)、sensor(传感器)。"
}

func (t *HAListDevicesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"domain": {
				"type": "string",
				"description": "设备类型过滤：light, switch, climate, fan, cover, sensor，为空则列出所有可控设备"
			}
		}
	}`)
}

type haListDevicesArgs struct {
	Domain string `json:"domain"`
}

func (t *HAListDevicesTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a haListDevicesArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	states, err := t.client.GetStates()
	if err != nil {
		return "", fmt.Errorf("获取设备列表失败: %w", err)
	}

	// 可控制的设备类型
	controllableDomains := map[string]bool{
		"light":   true,
		"switch":  true,
		"fan":     true,
		"climate": true,
		"cover":   true,
	}

	// 如果指定了 domain，只过滤该类型
	if a.Domain != "" {
		controllableDomains = map[string]bool{a.Domain: true}
	}

	var devices []string
	domainNames := map[string]string{
		"light":   "灯",
		"switch":  "开关",
		"fan":     "风扇/净化器",
		"climate": "空调",
		"cover":   "窗帘",
		"sensor":  "传感器",
	}

	for _, s := range states {
		parts := strings.SplitN(s.EntityID, ".", 2)
		if len(parts) != 2 {
			continue
		}
		domain := parts[0]

		// 如果是 sensor 类型请求，或者指定了 domain 且不在可控列表中，也返回
		if a.Domain == "sensor" || !controllableDomains[domain] {
			if a.Domain != "" && a.Domain != domain {
				continue
			}
		} else if !controllableDomains[domain] {
			continue
		}

		name := s.Attributes["friendly_name"]
		if name == nil {
			name = s.EntityID
		}

		domainName := domainNames[domain]
		if domainName == "" {
			domainName = domain
		}

		state := s.State
		if domain == "sensor" {
			if unit, ok := s.Attributes["unit_of_measurement"].(string); ok {
				state = state + unit
			}
		}

		devices = append(devices, fmt.Sprintf("- %s (%s) [%s]: %s", name, domainName, s.EntityID, state))
	}

	if len(devices) == 0 {
		return "没有找到设备。", nil
	}

	return "智能家居设备列表:\n" + strings.Join(devices, "\n"), nil
}

// HAGetDeviceStateTool 查询设备状态工具。
type HAGetDeviceStateTool struct {
	client *HomeAssistantClient
}

// NewHAGetDeviceStateTool 创建查询设备状态工具。
func NewHAGetDeviceStateTool(client *HomeAssistantClient) *HAGetDeviceStateTool {
	return &HAGetDeviceStateTool{client: client}
}

func (t *HAGetDeviceStateTool) Name() string {
	return "ha_get_device_state"
}

func (t *HAGetDeviceStateTool) Description() string {
	return "查询智能设备当前状态。**必须先调用 ha_list_devices 获取正确的 entity_id**，不要自己构造。如灯是否开着、空调温度、传感器数值等。"
}

func (t *HAGetDeviceStateTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"entity_id": {
				"type": "string",
				"description": "设备ID，如 light.ke_ting_deng"
			}
		},
		"required": ["entity_id"]
	}`)
}

type haGetDeviceStateArgs struct {
	EntityID string `json:"entity_id"`
}

func (t *HAGetDeviceStateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a haGetDeviceStateArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	state, err := t.client.GetState(a.EntityID)
	if err != nil {
		return "", fmt.Errorf("获取设备状态失败: %w", err)
	}

	name := state.Attributes["friendly_name"]
	if name == nil {
		name = a.EntityID
	}

	// 根据设备类型格式化状态
	parts := strings.SplitN(a.EntityID, ".", 2)
	domain := ""
	if len(parts) == 2 {
		domain = parts[0]
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s 当前状态: ", name))

	switch domain {
	case "light":
		if state.State == "on" {
			result.WriteString("已开启")
			if brightness, ok := state.Attributes["brightness"].(float64); ok {
				pct := int(brightness * 100 / 255)
				result.WriteString(fmt.Sprintf("，亮度 %d%%", pct))
			}
		} else {
			result.WriteString("已关闭")
		}
	case "switch":
		if state.State == "on" {
			result.WriteString("已开启")
		} else {
			result.WriteString("已关闭")
		}
	case "fan":
		if state.State == "on" {
			result.WriteString("运行中")
		} else {
			result.WriteString("已关闭")
		}
	case "climate":
		result.WriteString(state.State)
		if temp, ok := state.Attributes["temperature"].(float64); ok {
			result.WriteString(fmt.Sprintf("，设定温度 %.0f°C", temp))
		}
		if currentTemp, ok := state.Attributes["current_temperature"].(float64); ok {
			result.WriteString(fmt.Sprintf("，当前温度 %.1f°C", currentTemp))
		}
	case "sensor":
		unit, _ := state.Attributes["unit_of_measurement"].(string)
		result.WriteString(fmt.Sprintf("%s %s", state.State, unit))
	case "cover":
		result.WriteString(state.State)
	default:
		result.WriteString(state.State)
	}

	return result.String(), nil
}

// HAControlDeviceTool 控制设备工具。
type HAControlDeviceTool struct {
	client *HomeAssistantClient
}

// NewHAControlDeviceTool 创建控制设备工具。
func NewHAControlDeviceTool(client *HomeAssistantClient) *HAControlDeviceTool {
	return &HAControlDeviceTool{client: client}
}

func (t *HAControlDeviceTool) Name() string {
	return "ha_control_device"
}

func (t *HAControlDeviceTool) Description() string {
	return "控制智能家居设备。**必须先调用 ha_list_devices 获取正确的 entity_id**，不要自己构造。支持：开灯、关灯、调亮度、开空调、关空调、设温度、开风扇、关风扇等操作。"
}

func (t *HAControlDeviceTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"entity_id": {
				"type": "string",
				"description": "设备ID，如 light.xiaomi_light"
			},
			"action": {
				"type": "string",
				"enum": ["turn_on", "turn_off", "toggle", "set_brightness", "set_temperature"],
				"description": "操作类型：turn_on(开)、turn_off(关)、toggle(切换)、set_brightness(设亮度)、set_temperature(设温度)"
			},
			"value": {
				"type": "number",
				"description": "操作值：亮度1-100，温度(摄氏度)"
			}
		},
		"required": ["entity_id", "action"]
	}`)
}

type haControlDeviceArgs struct {
	EntityID string  `json:"entity_id"`
	Action   string  `json:"action"`
	Value    float64 `json:"value"`
}

func (t *HAControlDeviceTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a haControlDeviceArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	// 获取设备名称
	state, err := t.client.GetState(a.EntityID)
	if err != nil {
		return "", fmt.Errorf("设备不存在或无法访问: %w", err)
	}

	name := state.Attributes["friendly_name"]
	if name == nil {
		name = a.EntityID
	}

	// 解析设备类型
	parts := strings.SplitN(a.EntityID, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("无效的设备ID格式")
	}
	domain := parts[0]

	// 执行操作
	var actionName string
	switch a.Action {
	case "turn_on":
		if err := t.client.CallService(domain, "turn_on", map[string]interface{}{
			"entity_id": a.EntityID,
		}); err != nil {
			return "", fmt.Errorf("操作失败: %w", err)
		}
		actionName = "已开启"

	case "turn_off":
		if err := t.client.CallService(domain, "turn_off", map[string]interface{}{
			"entity_id": a.EntityID,
		}); err != nil {
			return "", fmt.Errorf("操作失败: %w", err)
		}
		actionName = "已关闭"

	case "toggle":
		if err := t.client.CallService(domain, "toggle", map[string]interface{}{
			"entity_id": a.EntityID,
		}); err != nil {
			return "", fmt.Errorf("操作失败: %w", err)
		}
		actionName = "已切换"

	case "set_brightness":
		if domain != "light" {
			return "", fmt.Errorf("只有灯光设备支持调节亮度")
		}
		brightness := int(a.Value * 255 / 100)
		if err := t.client.CallService(domain, "turn_on", map[string]interface{}{
			"entity_id":  a.EntityID,
			"brightness": brightness,
		}); err != nil {
			return "", fmt.Errorf("操作失败: %w", err)
		}
		actionName = fmt.Sprintf("亮度已设为 %.0f%%", a.Value)

	case "set_temperature":
		if domain != "climate" {
			return "", fmt.Errorf("只有空调设备支持设置温度")
		}
		if err := t.client.CallService(domain, "set_temperature", map[string]interface{}{
			"entity_id":   a.EntityID,
			"temperature": a.Value,
		}); err != nil {
			return "", fmt.Errorf("操作失败: %w", err)
		}
		actionName = fmt.Sprintf("温度已设为 %.0f°C", a.Value)

	default:
		return "", fmt.Errorf("不支持的操作: %s", a.Action)
	}

	logger.Infof("[tools] 控制设备: %s -> %s", a.EntityID, a.Action)
	return fmt.Sprintf("%s %s", name, actionName), nil
}
