package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/iabetor/pibuddy/internal/logger"
)

// VolumeController 音量控制器接口。
type VolumeController interface {
	GetVolume() (int, error)      // 获取当前音量 (0-100)
	SetVolume(volume int) error   // 设置音量 (0-100)
	IsMuted() (bool, error)       // 是否静音
	SetMute(muted bool) error     // 设置静音
}

// ---- macOS 实现 ----

type macosVolumeController struct{}

func newMacOSVolumeController() *macosVolumeController {
	return &macosVolumeController{}
}

func (c *macosVolumeController) GetVolume() (int, error) {
	cmd := exec.Command("osascript", "-e", "output volume of (get volume settings)")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("获取音量失败: %w", err)
	}
	vol, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, fmt.Errorf("解析音量失败: %w", err)
	}
	return vol, nil
}

func (c *macosVolumeController) SetVolume(volume int) error {
	if volume < 0 {
		volume = 0
	} else if volume > 100 {
		volume = 100
	}
	cmd := exec.Command("osascript", "-e", fmt.Sprintf("set volume output volume %d", volume))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("设置音量失败: %w", err)
	}
	return nil
}

func (c *macosVolumeController) IsMuted() (bool, error) {
	cmd := exec.Command("osascript", "-e", "output muted of (get volume settings)")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("获取静音状态失败: %w", err)
	}
	return strings.TrimSpace(string(output)) == "true", nil
}

func (c *macosVolumeController) SetMute(muted bool) error {
	var cmd *exec.Cmd
	if muted {
		cmd = exec.Command("osascript", "-e", "set volume output volume 0")
	} else {
		// 取消静音时恢复到50%
		cmd = exec.Command("osascript", "-e", "set volume output volume 50")
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("设置静音失败: %w", err)
	}
	return nil
}

// ---- Linux 实现 (PulseAudio) ----

type pulseAudioController struct {
	cachedSink string
}

func newPulseAudioController() *pulseAudioController {
	return &pulseAudioController{}
}

func (c *pulseAudioController) getDefaultSink() (string, error) {
	if c.cachedSink != "" {
		return c.cachedSink, nil
	}
	cmd := exec.Command("pactl", "get-default-sink")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	c.cachedSink = strings.TrimSpace(string(output))
	return c.cachedSink, nil
}

func (c *pulseAudioController) GetVolume() (int, error) {
	sink, err := c.getDefaultSink()
	if err != nil {
		return 0, fmt.Errorf("获取默认音源失败: %w", err)
	}
	cmd := exec.Command("pactl", "get-sink-volume", sink)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("获取音量失败: %w", err)
	}
	// 输出格式: Volume: front-left: 12345 /  19% / -42.34 dB, ...
	// 提取第一个百分比
	parts := strings.Split(string(output), "%")
	if len(parts) < 1 {
		return 0, fmt.Errorf("无法解析音量输出")
	}
	volStr := parts[0]
	// 找到最后一个数字
	fields := strings.Fields(volStr)
	for i := len(fields) - 1; i >= 0; i-- {
		if vol, err := strconv.Atoi(fields[i]); err == nil {
			return vol, nil
		}
	}
	return 0, fmt.Errorf("无法解析音量值")
}

func (c *pulseAudioController) SetVolume(volume int) error {
	if volume < 0 {
		volume = 0
	} else if volume > 100 {
		volume = 100
	}
	sink, err := c.getDefaultSink()
	if err != nil {
		return fmt.Errorf("获取默认音源失败: %w", err)
	}
	cmd := exec.Command("pactl", "set-sink-volume", sink, fmt.Sprintf("%d%%", volume))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("设置音量失败: %w", err)
	}
	return nil
}

func (c *pulseAudioController) IsMuted() (bool, error) {
	sink, err := c.getDefaultSink()
	if err != nil {
		return false, fmt.Errorf("获取默认音源失败: %w", err)
	}
	cmd := exec.Command("pactl", "get-sink-mute", sink)
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("获取静音状态失败: %w", err)
	}
	return strings.Contains(string(output), "yes"), nil
}

func (c *pulseAudioController) SetMute(muted bool) error {
	sink, err := c.getDefaultSink()
	if err != nil {
		return fmt.Errorf("获取默认音源失败: %w", err)
	}
	var muteVal string
	if muted {
		muteVal = "1"
	} else {
		muteVal = "0"
	}
	cmd := exec.Command("pactl", "set-sink-mute", sink, muteVal)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("设置静音失败: %w", err)
	}
	return nil
}

// ---- Linux 实现 (ALSA 回退) ----

type alsaController struct{}

func newALSAController() *alsaController {
	return &alsaController{}
}

func (c *alsaController) GetVolume() (int, error) {
	cmd := exec.Command("amixer", "get", "Master")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("获取音量失败: %w", err)
	}
	// 输出格式: Mono: Playback 39 [60%] [-16.00dB] [on]
	// 提取百分比
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "%") {
			start := strings.Index(line, "[")
			end := strings.Index(line, "%")
			if start != -1 && end != -1 && end > start {
				volStr := line[start+1 : end]
				vol, err := strconv.Atoi(volStr)
				if err == nil {
					return vol, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("无法解析音量值")
}

func (c *alsaController) SetVolume(volume int) error {
	if volume < 0 {
		volume = 0
	} else if volume > 100 {
		volume = 100
	}
	cmd := exec.Command("amixer", "set", "Master", fmt.Sprintf("%d%%", volume))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("设置音量失败: %w", err)
	}
	return nil
}

func (c *alsaController) IsMuted() (bool, error) {
	cmd := exec.Command("amixer", "get", "Master")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("获取静音状态失败: %w", err)
	}
	return strings.Contains(string(output), "[off]"), nil
}

func (c *alsaController) SetMute(muted bool) error {
	var cmd *exec.Cmd
	if muted {
		cmd = exec.Command("amixer", "set", "Master", "mute")
	} else {
		cmd = exec.Command("amixer", "set", "Master", "unmute")
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("设置静音失败: %w", err)
	}
	return nil
}

// NewVolumeController 创建适合当前系统的音量控制器。
func NewVolumeController() (VolumeController, error) {
	switch runtime.GOOS {
	case "darwin":
		logger.Info("[tools] 使用 macOS 音量控制器")
		return newMacOSVolumeController(), nil
	case "linux":
		// 优先尝试 PulseAudio
		if _, err := exec.LookPath("pactl"); err == nil {
			logger.Info("[tools] 使用 PulseAudio 音量控制器")
			return newPulseAudioController(), nil
		}
		// 回退到 ALSA
		if _, err := exec.LookPath("amixer"); err == nil {
			logger.Info("[tools] 使用 ALSA 音量控制器")
			return newALSAController(), nil
		}
		return nil, fmt.Errorf("未找到可用的音量控制系统（需要 pactl 或 amixer）")
	default:
		return nil, fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}

// ---- SetVolumeTool ----

type SetVolumeTool struct {
	controller VolumeController
	step       int // 相对调节步长
}

type VolumeConfig struct {
	Step int // 相对调节步长，默认 10
}

func NewSetVolumeTool(controller VolumeController, cfg VolumeConfig) *SetVolumeTool {
	step := cfg.Step
	if step <= 0 {
		step = 10
	}
	return &SetVolumeTool{controller: controller, step: step}
}

func (t *SetVolumeTool) Name() string { return "set_volume" }
func (t *SetVolumeTool) Description() string {
	return "设置播放音量。当用户说'音量设为X'、'调大音量'、'调小音量'、'静音'时使用。"
}
func (t *SetVolumeTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"volume": {
				"type": "integer",
				"description": "音量值 (0-100)，-1 表示切换静音"
			},
			"relative": {
				"type": "boolean",
				"description": "是否相对调节（true时volume为增量，可为负数）"
			}
		},
		"required": ["volume"]
	}`)
}

type setVolumeArgs struct {
	Volume   int  `json:"volume"`
	Relative bool `json:"relative"`
}

func (t *SetVolumeTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a setVolumeArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// -1 表示切换静音
	if a.Volume == -1 {
		muted, err := t.controller.IsMuted()
		if err != nil {
			return "", err
		}
		if err := t.controller.SetMute(!muted); err != nil {
			return "", err
		}
		if !muted {
			return "已静音", nil
		}
		return "已取消静音", nil
	}

	var newVolume int
	if a.Relative {
		// 相对调节
		current, err := t.controller.GetVolume()
		if err != nil {
			return "", err
		}
		newVolume = current + a.Volume
	} else {
		// 绝对设置
		newVolume = a.Volume
	}

	// 确保在有效范围内
	if newVolume < 0 {
		newVolume = 0
	} else if newVolume > 100 {
		newVolume = 100
	}

	if err := t.controller.SetVolume(newVolume); err != nil {
		return "", err
	}

	// 检查静音状态
	muted, _ := t.controller.IsMuted()
	if muted {
		// 取消静音
		_ = t.controller.SetMute(false)
	}

	return fmt.Sprintf("音量已设为%d", newVolume), nil
}

// ---- GetVolumeTool ----

type GetVolumeTool struct {
	controller VolumeController
}

func NewGetVolumeTool(controller VolumeController) *GetVolumeTool {
	return &GetVolumeTool{controller: controller}
}

func (t *GetVolumeTool) Name() string { return "get_volume" }
func (t *GetVolumeTool) Description() string {
	return "查询当前音量。当用户说'现在音量是多少'、'当前音量'时使用。"
}
func (t *GetVolumeTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}

func (t *GetVolumeTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	muted, err := t.controller.IsMuted()
	if err != nil {
		return "", err
	}
	if muted {
		return "当前已静音", nil
	}

	volume, err := t.controller.GetVolume()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("当前音量是%d", volume), nil
}
