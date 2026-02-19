package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
)

// EzvizClient 萤石开放平台 API 客户端。
type EzvizClient struct {
	appKey     string
	appSecret  string
	httpClient *http.Client

	mu          sync.Mutex
	accessToken string
	expireTime  time.Time
}

// NewEzvizClient 创建萤石客户端。
func NewEzvizClient(appKey, appSecret string) *EzvizClient {
	return &EzvizClient{
		appKey:    appKey,
		appSecret: appSecret,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ezvizResponse 萤石 API 通用响应。
type ezvizResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// tokenData accessToken 响应数据。
type tokenData struct {
	AccessToken string `json:"accessToken"`
	ExpireTime  int64  `json:"expireTime"`
}

// EzvizDeviceInfo 设备信息。
type EzvizDeviceInfo struct {
	DeviceSerial   string `json:"deviceSerial"`
	DeviceName     string `json:"deviceName"`
	Model          string `json:"model"`
	Status         int    `json:"status"`
	Category       string `json:"category"`
	ParentCategory string `json:"parentCategory"`
	NetType        string `json:"netType"`
	Signal         string `json:"signal"`
}

// EzvizDeviceCapacity 设备能力集。
type EzvizDeviceCapacity struct {
	SupportRemoteOpenDoor    string `json:"support_remote_open_door"`
	SupportRemoteAuthRandcode string `json:"support_remote_auth_randcode"`
	SupportCheckDoorState    string `json:"support_check_door_state"`
	SupportLockBatteryPerCent string `json:"support_lock_battery_per_cent"`
	SupportLockVolumeSetting string `json:"support_lock_volume_setting"`
}

// getAccessToken 获取或刷新 accessToken。
func (c *EzvizClient) getAccessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果 token 还有效（提前 5 分钟刷新）
	if c.accessToken != "" && time.Now().Before(c.expireTime.Add(-5*time.Minute)) {
		return c.accessToken, nil
	}

	form := url.Values{
		"appKey":    {c.appKey},
		"appSecret": {c.appSecret},
	}

	resp, err := c.httpClient.PostForm("https://open.ys7.com/api/lapp/token/get", form)
	if err != nil {
		return "", fmt.Errorf("请求 accessToken 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var r ezvizResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if r.Code != "200" {
		return "", fmt.Errorf("获取 accessToken 失败: %s (%s)", r.Msg, r.Code)
	}

	var td tokenData
	if err := json.Unmarshal(r.Data, &td); err != nil {
		return "", fmt.Errorf("解析 token 数据失败: %w", err)
	}

	c.accessToken = td.AccessToken
	c.expireTime = time.UnixMilli(td.ExpireTime)

	logger.Infof("[ezviz] accessToken 已刷新，有效期至 %s", c.expireTime.Format("2006-01-02 15:04:05"))
	return c.accessToken, nil
}

// doPost 执行 POST 请求。
func (c *EzvizClient) doPost(apiPath string, params url.Values) (*ezvizResponse, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	if params == nil {
		params = url.Values{}
	}
	params.Set("accessToken", token)

	resp, err := c.httpClient.PostForm("https://open.ys7.com"+apiPath, params)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var r ezvizResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &r, nil
}

// GetDeviceInfo 获取设备信息。
func (c *EzvizClient) GetDeviceInfo(deviceSerial string) (*EzvizDeviceInfo, error) {
	r, err := c.doPost("/api/lapp/device/info", url.Values{
		"deviceSerial": {deviceSerial},
	})
	if err != nil {
		return nil, err
	}
	if r.Code != "200" {
		return nil, fmt.Errorf("获取设备信息失败: %s (%s)", r.Msg, r.Code)
	}

	var info EzvizDeviceInfo
	if err := json.Unmarshal(r.Data, &info); err != nil {
		return nil, fmt.Errorf("解析设备信息失败: %w", err)
	}
	return &info, nil
}

// GetDeviceCapacity 获取设备能力集。
func (c *EzvizClient) GetDeviceCapacity(deviceSerial string) (*EzvizDeviceCapacity, error) {
	r, err := c.doPost("/api/lapp/device/capacity", url.Values{
		"deviceSerial": {deviceSerial},
	})
	if err != nil {
		return nil, err
	}
	if r.Code != "200" {
		return nil, fmt.Errorf("获取设备能力失败: %s (%s)", r.Msg, r.Code)
	}

	var cap EzvizDeviceCapacity
	if err := json.Unmarshal(r.Data, &cap); err != nil {
		return nil, fmt.Errorf("解析设备能力失败: %w", err)
	}
	return &cap, nil
}

// ListDevices 获取设备列表。
func (c *EzvizClient) ListDevices() ([]EzvizDeviceInfo, error) {
	r, err := c.doPost("/api/lapp/device/list", url.Values{
		"pageStart": {"0"},
		"pageSize":  {"50"},
	})
	if err != nil {
		return nil, err
	}
	if r.Code != "200" {
		return nil, fmt.Errorf("获取设备列表失败: %s (%s)", r.Msg, r.Code)
	}

	var devices []EzvizDeviceInfo
	if err := json.Unmarshal(r.Data, &devices); err != nil {
		return nil, fmt.Errorf("解析设备列表失败: %w", err)
	}
	return devices, nil
}

// RemoteOpenDoor 远程开门（通过 SaaS 组件接口）。
func (c *EzvizClient) RemoteOpenDoor(deviceSerial string) error {
	r, err := c.doPost("/api/component/saas/smartlock/remote/door", url.Values{
		"deviceSerial": {deviceSerial},
		"cmd":          {"open"},
	})
	if err != nil {
		return err
	}
	if r.Code != "200" {
		return fmt.Errorf("远程开锁失败: %s (%s)", r.Msg, r.Code)
	}
	return nil
}

// --- 工具定义 ---

// EzvizListDevicesTool 列出萤石设备工具。
type EzvizListDevicesTool struct {
	client *EzvizClient
}

func NewEzvizListDevicesTool(client *EzvizClient) *EzvizListDevicesTool {
	return &EzvizListDevicesTool{client: client}
}

func (t *EzvizListDevicesTool) Name() string { return "ezviz_list_devices" }

func (t *EzvizListDevicesTool) Description() string {
	return "列出所有萤石智能设备（门锁、摄像头等）。返回设备名称、序列号、在线状态、电量等信息。"
}

func (t *EzvizListDevicesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

func (t *EzvizListDevicesTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	devices, err := t.client.ListDevices()
	if err != nil {
		return "", fmt.Errorf("获取设备列表失败: %w", err)
	}

	if len(devices) == 0 {
		return "没有找到萤石设备。", nil
	}

	var lines []string
	categoryNames := map[string]string{
		"VideoLock": "视频锁",
		"IPC":       "摄像头",
		"DVR":       "录像机",
	}

	for _, d := range devices {
		statusStr := "离线"
		if d.Status == 1 {
			statusStr = "在线"
		}

		catName := categoryNames[d.ParentCategory]
		if catName == "" {
			catName = d.ParentCategory
		}

		lines = append(lines, fmt.Sprintf("- %s (%s) [序列号: %s] 状态: %s",
			d.DeviceName, catName, d.DeviceSerial, statusStr))
	}

	return "萤石设备列表:\n" + strings.Join(lines, "\n"), nil
}

// EzvizGetLockStatusTool 查询门锁状态工具。
type EzvizGetLockStatusTool struct {
	client       *EzvizClient
	deviceSerial string // 默认门锁序列号
}

func NewEzvizGetLockStatusTool(client *EzvizClient, deviceSerial string) *EzvizGetLockStatusTool {
	return &EzvizGetLockStatusTool{client: client, deviceSerial: deviceSerial}
}

func (t *EzvizGetLockStatusTool) Name() string { return "ezviz_lock_status" }

func (t *EzvizGetLockStatusTool) Description() string {
	return "查询萤石门锁状态。返回门锁在线状态、电量、信号强度等信息。注意：萤石开放平台不提供门开关状态的查询接口，门的实时开关状态只能通过萤石云APP查看。不传序列号则查询默认门锁。"
}

func (t *EzvizGetLockStatusTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"device_serial": {
				"type": "string",
				"description": "门锁序列号，不传则使用默认门锁"
			}
		}
	}`)
}

type ezvizLockStatusArgs struct {
	DeviceSerial string `json:"device_serial"`
}

func (t *EzvizGetLockStatusTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a ezvizLockStatusArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	serial := a.DeviceSerial
	if serial == "" {
		serial = t.deviceSerial
	}
	if serial == "" {
		return "", fmt.Errorf("未指定门锁序列号")
	}

	info, err := t.client.GetDeviceInfo(serial)
	if err != nil {
		return "", fmt.Errorf("获取门锁信息失败: %w", err)
	}

	statusStr := "离线"
	if info.Status == 1 {
		statusStr = "在线"
	}

	result := fmt.Sprintf("门锁 %s 状态:\n- 设备: %s (%s)\n- 状态: %s\n- 网络: %s (信号 %s)",
		info.DeviceName, info.Model, info.Category,
		statusStr, info.NetType, info.Signal)

	logger.Infof("[ezviz] 查询门锁状态: %s -> %s", serial, statusStr)
	return result, nil
}

// EzvizOpenDoorTool 远程开锁工具。
type EzvizOpenDoorTool struct {
	client       *EzvizClient
	deviceSerial string
}

func NewEzvizOpenDoorTool(client *EzvizClient, deviceSerial string) *EzvizOpenDoorTool {
	return &EzvizOpenDoorTool{client: client, deviceSerial: deviceSerial}
}

func (t *EzvizOpenDoorTool) Name() string { return "ezviz_open_door" }

func (t *EzvizOpenDoorTool) Description() string {
	return "远程打开萤石门锁。注意：此操作会真实开锁，请确认用户意图。不传序列号则操作默认门锁。"
}

func (t *EzvizOpenDoorTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"device_serial": {
				"type": "string",
				"description": "门锁序列号，不传则使用默认门锁"
			},
			"confirm": {
				"type": "boolean",
				"description": "确认开锁，必须为 true 才执行"
			}
		},
		"required": ["confirm"]
	}`)
}

type ezvizOpenDoorArgs struct {
	DeviceSerial string `json:"device_serial"`
	Confirm      bool   `json:"confirm"`
}

func (t *EzvizOpenDoorTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a ezvizOpenDoorArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	if !a.Confirm {
		return "开锁操作需要确认。请再次说「确认开锁」来执行。", nil
	}

	serial := a.DeviceSerial
	if serial == "" {
		serial = t.deviceSerial
	}
	if serial == "" {
		return "", fmt.Errorf("未指定门锁序列号")
	}

	// 先检查设备是否在线
	info, err := t.client.GetDeviceInfo(serial)
	if err != nil {
		return "", fmt.Errorf("获取门锁信息失败: %w", err)
	}
	if info.Status != 1 {
		return fmt.Sprintf("门锁 %s 当前离线，无法远程开锁。", info.DeviceName), nil
	}

	// 执行远程开锁
	if err := t.client.RemoteOpenDoor(serial); err != nil {
		return "", fmt.Errorf("远程开锁失败: %w", err)
	}

	logger.Infof("[ezviz] 远程开锁成功: %s (%s)", serial, info.DeviceName)
	return fmt.Sprintf("门锁 %s 已远程开锁。", info.DeviceName), nil
}
