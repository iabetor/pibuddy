package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/iabetor/pibuddy/internal/audio"
	"github.com/iabetor/pibuddy/internal/config"
	"github.com/iabetor/pibuddy/internal/voiceprint"
)

func main() {
	configPath := flag.String("config", "configs/pibuddy.yaml", "配置文件路径")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	if !cfg.Voiceprint.Enabled {
		fmt.Fprintln(os.Stderr, "声纹识别未启用，请在配置文件中设置 voiceprint.enabled: true")
		os.Exit(1)
	}

	mgr, err := voiceprint.NewManager(cfg.Voiceprint, cfg.Tools.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化声纹管理器失败: %v\n", err)
		os.Exit(1)
	}
	defer mgr.Close()

	switch args[0] {
	case "register":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "用法: pibuddy-user register <用户名>")
			os.Exit(1)
		}
		cmdRegister(mgr, cfg, args[1])
	case "list":
		cmdList(mgr)
	case "delete":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "用法: pibuddy-user delete <用户名>")
			os.Exit(1)
		}
		cmdDelete(mgr, args[1])
	case "set-owner":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "用法: pibuddy-user set-owner <用户名>")
			os.Exit(1)
		}
		cmdSetOwner(mgr, args[1])
	case "set-prefs":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "用法: pibuddy-user set-prefs <用户名> <偏好JSON>")
			fmt.Fprintln(os.Stderr, "示例: pibuddy-user set-prefs 张三 '{\"style\":\"简洁直接\",\"interests\":[\"编程\"]}'")
			os.Exit(1)
		}
		cmdSetPrefs(mgr, args[1], args[2])
	case "get-prefs":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "用法: pibuddy-user get-prefs <用户名>")
			os.Exit(1)
		}
		cmdGetPrefs(mgr, args[1])
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "PiBuddy 声纹用户管理工具")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "用法: pibuddy-user [-config <path>] <command> [args]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "命令:")
	fmt.Fprintln(os.Stderr, "  register <用户名>      录制声纹并注册用户（需要 3 个 3 秒样本）")
	fmt.Fprintln(os.Stderr, "  list                  列出所有已注册的声纹用户")
	fmt.Fprintln(os.Stderr, "  delete <用户名>        删除用户及其声纹数据")
	fmt.Fprintln(os.Stderr, "  set-owner <用户名>     设置用户为主人")
	fmt.Fprintln(os.Stderr, "  set-prefs <用户名> <JSON>  设置用户偏好")
	fmt.Fprintln(os.Stderr, "  get-prefs <用户名>     获取用户偏好")
}

func cmdRegister(mgr *voiceprint.Manager, cfg *config.Config, name string) {
	const numSamples = 3
	const sampleDuration = 3 * time.Second

	capture, err := audio.NewCapture(cfg.Audio.SampleRate, cfg.Audio.Channels, cfg.Audio.FrameSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化麦克风失败: %v\n", err)
		os.Exit(1)
	}
	defer capture.Close()

	if err := capture.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "启动麦克风失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("即将为用户 [%s] 注册声纹，需要录制 %d 个 %v 的语音样本。\n", name, numSamples, sampleDuration)
	fmt.Println("请在每次提示后开始说话（可以说任意内容）。")
	fmt.Println()

	var samples [][]float32
	for i := 0; i < numSamples; i++ {
		fmt.Printf("第 %d/%d 个样本 — 按回车开始录制...", i+1, numSamples)
		fmt.Scanln()
		fmt.Printf("  录制中（%v）...\n", sampleDuration)

		ctx, cancel := context.WithTimeout(context.Background(), sampleDuration)
		recorded := capture.RecordFor(ctx)
		cancel()

		if len(recorded) < cfg.Audio.SampleRate {
			fmt.Fprintln(os.Stderr, "  录制数据不足，请重试。")
			i--
			continue
		}

		samples = append(samples, recorded)
		fmt.Printf("  已录制 %d 个采样点\n", len(recorded))
	}

	fmt.Print("正在注册...")
	if err := mgr.Register(name, samples); err != nil {
		fmt.Fprintf(os.Stderr, "\n注册失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(" 完成！")
	log.Printf("用户 %s 注册成功", name)
}

func cmdList(mgr *voiceprint.Manager) {
	users, err := mgr.ListUsers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "列出用户失败: %v\n", err)
		os.Exit(1)
	}

	if len(users) == 0 {
		fmt.Println("当前没有已注册的声纹用户。")
		return
	}

	fmt.Printf("已注册 %d 个声纹用户:\n", len(users))
	fmt.Println("  名称       | 主人 | 偏好")
	fmt.Println("  -----------+------+------")
	for _, u := range users {
		ownerMark := "  "
		if u.IsOwner() {
			ownerMark = "✓ "
		}
		prefs := u.GetPreferences()
		if prefs == "" {
			prefs = "(无)"
		}
		fmt.Printf("  %-10s | %s  | %s\n", u.Name, ownerMark, prefs)
	}
}

func cmdDelete(mgr *voiceprint.Manager, name string) {
	if err := mgr.DeleteUser(name); err != nil {
		fmt.Fprintf(os.Stderr, "删除失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("用户 %s 已删除。\n", name)
}

func cmdSetOwner(mgr *voiceprint.Manager, name string) {
	if err := mgr.SetOwner(name); err != nil {
		fmt.Fprintf(os.Stderr, "设置主人失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("已将 %s 设置为主人。\n", name)
}

func cmdSetPrefs(mgr *voiceprint.Manager, name, prefsJSON string) {
	// 验证 JSON 格式
	var prefs voiceprint.UserPreferences
	if err := json.Unmarshal([]byte(prefsJSON), &prefs); err != nil {
		fmt.Fprintf(os.Stderr, "JSON 格式错误: %v\n", err)
		os.Exit(1)
	}

	// 存储（直接存储原始 JSON）
	if err := mgr.SetPreferences(name, prefsJSON); err != nil {
		fmt.Fprintf(os.Stderr, "设置偏好失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("已为 %s 设置偏好。\n", name)
}

func cmdGetPrefs(mgr *voiceprint.Manager, name string) {
	user, err := mgr.GetUser(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "查询用户失败: %v\n", err)
		os.Exit(1)
	}
	if user == nil {
		fmt.Fprintf(os.Stderr, "用户 %s 不存在\n", name)
		os.Exit(1)
	}

	prefs := user.GetPreferences()
	if prefs == "" {
		fmt.Printf("用户 %s 没有设置偏好。\n", name)
		return
	}

	// 格式化输出
	var p voiceprint.UserPreferences
	if err := json.Unmarshal([]byte(prefs), &p); err != nil {
		fmt.Printf("用户 %s 的偏好（原始）: %s\n", name, prefs)
		return
	}

	fmt.Printf("用户 %s 的偏好:\n", name)
	if p.Style != "" {
		fmt.Printf("  回复风格: %s\n", p.Style)
	}
	if p.Nickname != "" {
		fmt.Printf("  昵称: %s\n", p.Nickname)
	}
	if len(p.Interests) > 0 {
		fmt.Printf("  兴趣: %v\n", p.Interests)
	}
	if p.Extra != "" {
		fmt.Printf("  额外信息: %s\n", p.Extra)
	}
}
