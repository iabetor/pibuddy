package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/iabetor/pibuddy/internal/logger"
)

// SystemStatusTool 系统状态查询工具。
type SystemStatusTool struct{}

// NewSystemStatusTool 创建系统状态工具。
func NewSystemStatusTool() *SystemStatusTool {
	return &SystemStatusTool{}
}

func (t *SystemStatusTool) Name() string {
	return "get_system_status"
}

func (t *SystemStatusTool) Description() string {
	return "获取系统状态，包括CPU温度、内存使用、磁盘空间、电池电量等。"
}

func (t *SystemStatusTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

func (t *SystemStatusTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var results []string

	// CPU 温度
	if temp := t.getCPUTemperature(); temp != "" {
		results = append(results, fmt.Sprintf("CPU温度: %s", temp))
	}

	// 内存使用
	if mem := t.getMemoryUsage(); mem != "" {
		results = append(results, fmt.Sprintf("内存: %s", mem))
	}

	// 磁盘空间
	if disk := t.getDiskUsage(); disk != "" {
		results = append(results, fmt.Sprintf("磁盘: %s", disk))
	}

	// 电池电量
	if battery := t.getBatteryLevel(); battery != "" {
		results = append(results, fmt.Sprintf("电池: %s", battery))
	}

	// CPU 使用率
	if cpu := t.getCPUUsage(); cpu != "" {
		results = append(results, fmt.Sprintf("CPU: %s", cpu))
	}

	// 运行时间
	if uptime := t.getUptime(); uptime != "" {
		results = append(results, fmt.Sprintf("运行时间: %s", uptime))
	}

	if len(results) == 0 {
		return "无法获取系统状态", nil
	}

	return strings.Join(results, "；"), nil
}

// getCPUTemperature 获取 CPU 温度。
func (t *SystemStatusTool) getCPUTemperature() string {
	// 树莓派: 读取 thermal_zone
	if data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp"); err == nil {
		tempStr := strings.TrimSpace(string(data))
		if temp, err := strconv.ParseFloat(tempStr, 64); err == nil {
			return fmt.Sprintf("%.1f°C", temp/1000)
		}
	}

	// Linux: 尝试 sensors 命令
	if runtime.GOOS == "linux" {
		if output, err := exec.Command("sensors", "-j").Output(); err == nil {
			// 简单解析，寻找 temp1_input
			var sensors map[string]interface{}
			if json.Unmarshal(output, &sensors) == nil {
				for _, adapter := range sensors {
					if adapterMap, ok := adapter.(map[string]interface{}); ok {
						for _, sensor := range adapterMap {
							if sensorMap, ok := sensor.(map[string]interface{}); ok {
								if temp, ok := sensorMap["temp1_input"].(float64); ok {
									return fmt.Sprintf("%.1f°C", temp)
								}
							}
						}
					}
				}
			}
		}
	}

	// macOS: 使用 osx-cpu-temp 或 powermetrics
	if runtime.GOOS == "darwin" {
		// 尝试 powermetrics (需要 sudo，跳过)
		// 返回空，macOS 没有简单的 CPU 温度获取方式
	}

	return ""
}

// getMemoryUsage 获取内存使用情况。
func (t *SystemStatusTool) getMemoryUsage() string {
	// Linux (包括树莓派)
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			lines := strings.Split(string(data), "\n")
			var total, available uint64
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					value, _ := strconv.ParseUint(fields[1], 10, 64)
					switch fields[0] {
					case "MemTotal:":
						total = value
					case "MemAvailable:":
						available = value
					case "MemFree:": // 回退
						if available == 0 {
							available = value
						}
					}
				}
			}
			if total > 0 {
				used := total - available
				usedPercent := float64(used) / float64(total) * 100
				usedMB := used / 1024
				totalMB := total / 1024
				return fmt.Sprintf("%.0f%% (%d/%d MB)", usedPercent, usedMB, totalMB)
			}
		}
	}

	// macOS: 使用 vm_stat
	if runtime.GOOS == "darwin" {
		if output, err := exec.Command("vm_stat").Output(); err == nil {
			var pageSize uint64 = 4096 // macOS 默认页面大小
			var active, inactive uint64

			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					valueStr := strings.TrimSuffix(fields[2], ".")
					value, _ := strconv.ParseUint(valueStr, 10, 64)
					switch fields[0] {
					case "Pages":
						if len(fields) >= 2 {
							switch fields[1] {
							case "active:":
								active = value
							case "inactive:":
								inactive = value
							}
						}
					}
				}
			}

			// 获取总内存
			if totalOutput, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
				totalStr := strings.TrimSpace(string(totalOutput))
				total, _ := strconv.ParseUint(totalStr, 10, 64)
				totalMB := total / 1024 / 1024

				used := (active + inactive) * pageSize
				usedMB := used / 1024 / 1024
				usedPercent := float64(used) / float64(total) * 100

				return fmt.Sprintf("%.0f%% (%d/%d MB)", usedPercent, usedMB, totalMB)
			}
		}
	}

	return ""
}

// getDiskUsage 获取磁盘使用情况。
func (t *SystemStatusTool) getDiskUsage() string {
	// 使用 df 命令 (Linux 和 macOS 都支持)
	output, err := exec.Command("df", "-h", "/").Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) >= 2 {
		fields := strings.Fields(lines[1])
		if len(fields) >= 5 {
			// fields: Filesystem Size Used Avail Use% Mounted
			return fmt.Sprintf("已用 %s/%s (%s)", fields[2], fields[1], fields[4])
		}
	}

	return ""
}

// getBatteryLevel 获取电池电量。
func (t *SystemStatusTool) getBatteryLevel() string {
	// Linux (树莓派使用 UPS HAT 等)
	if runtime.GOOS == "linux" {
		// 尝试读取电池信息
		batteryPaths := []string{
			"/sys/class/power_supply/BAT0/capacity",
			"/sys/class/power_supply/BAT1/capacity",
			"/sys/class/power_supply/battery/capacity",
		}

		for _, path := range batteryPaths {
			if data, err := os.ReadFile(path); err == nil {
				level := strings.TrimSpace(string(data))
				// 获取充电状态
				status := ""
				statusPath := strings.Replace(path, "capacity", "status", 1)
				if statusData, err := os.ReadFile(statusPath); err == nil {
					status = strings.TrimSpace(string(statusData))
					if status == "Charging" {
						status = "充电中"
					} else if status == "Discharging" {
						status = "放电中"
					} else if status == "Full" {
						status = "已充满"
					}
				}
				if status != "" {
					return fmt.Sprintf("%s%% (%s)", level, status)
				}
				return level + "%"
			}
		}
	}

	// macOS: 使用 pmset
	if runtime.GOOS == "darwin" {
		if output, err := exec.Command("pmset", "-g", "batt").Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "%") {
					// 解析类似: -InternalBattery-0 (id=123) 100%; charged; 0:00 remaining
					if idx := strings.Index(line, "%"); idx > 0 {
						// 找百分比
						start := idx - 3
						if start < 0 {
							start = 0
						}
						percent := strings.TrimSpace(line[start:idx])

						// 找状态
						status := ""
						if strings.Contains(line, "charging") {
							status = "充电中"
						} else if strings.Contains(line, "discharging") {
							status = "放电中"
						} else if strings.Contains(line, "charged") {
							status = "已充满"
						}

						if status != "" {
							return fmt.Sprintf("%s%% (%s)", percent, status)
						}
						return percent + "%"
					}
				}
			}
		}
	}

	return ""
}

// getCPUUsage 获取 CPU 使用率。
func (t *SystemStatusTool) getCPUUsage() string {
	// Linux: 读取 /proc/stat
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/stat"); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "cpu ") {
					fields := strings.Fields(line)
					if len(fields) >= 8 {
						// user, nice, system, idle, iowait, irq, softirq, steal
						user, _ := strconv.ParseFloat(fields[1], 64)
						nice, _ := strconv.ParseFloat(fields[2], 64)
						system, _ := strconv.ParseFloat(fields[3], 64)
						idle, _ := strconv.ParseFloat(fields[4], 64)
						iowait, _ := strconv.ParseFloat(fields[5], 64)
						irq, _ := strconv.ParseFloat(fields[6], 64)
						softirq, _ := strconv.ParseFloat(fields[7], 64)

						total := user + nice + system + idle + iowait + irq + softirq
						used := user + nice + system + irq + softirq
						if total > 0 {
							percent := used / total * 100
							return fmt.Sprintf("%.0f%%", percent)
						}
					}
					break
				}
			}
		}
	}

	// macOS: 使用 top 命令
	if runtime.GOOS == "darwin" {
		if output, err := exec.Command("sh", "-c", "top -l 1 | grep 'CPU usage'").Output(); err == nil {
			// 输出类似: CPU usage: 5.26% user, 10.52% sys, 84.21% idle
			line := strings.TrimSpace(string(output))
			if strings.Contains(line, "idle") {
				parts := strings.Split(line, ",")
				for _, part := range parts {
					if strings.Contains(part, "idle") {
						fields := strings.Fields(part)
						if len(fields) >= 1 {
							idleStr := strings.TrimSuffix(fields[0], "%")
							if idle, err := strconv.ParseFloat(idleStr, 64); err == nil {
								return fmt.Sprintf("%.0f%%", 100-idle)
							}
						}
					}
				}
			}
		}
	}

	return ""
}

// getUptime 获取系统运行时间。
func (t *SystemStatusTool) getUptime() string {
	// Linux: 读取 /proc/uptime
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/uptime"); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) >= 1 {
				uptime, _ := strconv.ParseFloat(fields[0], 64)
				return formatUptime(uptime)
			}
		}
	}

	// macOS: 使用 uptime 命令
	if runtime.GOOS == "darwin" {
		if output, err := exec.Command("uptime").Output(); err == nil {
			// 输出类似: 23:30  up 1 day, 5:20, 2 users, load averages: 1.20 1.15 1.10
			line := string(output)
			// 简单提取
			if idx := strings.Index(line, "up "); idx >= 0 {
				rest := line[idx+3:]
				if endIdx := strings.Index(rest, ","); endIdx > 0 {
					return strings.TrimSpace(rest[:endIdx])
				}
				if endIdx := strings.Index(rest, " user"); endIdx > 0 {
					return strings.TrimSpace(rest[:endIdx])
				}
			}
		}
	}

	return ""
}

// formatUptime 格式化运行时间。
func formatUptime(seconds float64) string {
	days := int(seconds) / 86400
	hours := (int(seconds) % 86400) / 3600
	mins := (int(seconds) % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%d天%d小时", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, mins)
	}
	return fmt.Sprintf("%d分钟", mins)
}

// init 注册工具。
func init() {
	// 工具会在 pipeline 中注册
	logger.Debug("[tools] system 工具已加载")
}
