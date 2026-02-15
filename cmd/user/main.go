package main

import (
	"context"
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
	fmt.Fprintln(os.Stderr, "  register <用户名>  录制声纹并注册用户（需要 3 个 3 秒样本）")
	fmt.Fprintln(os.Stderr, "  list              列出所有已注册的声纹用户")
	fmt.Fprintln(os.Stderr, "  delete <用户名>    删除用户及其声纹数据")
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
	fmt.Println("  ID  | 名称")
	fmt.Println("  ----+----------")
	for _, u := range users {
		fmt.Printf("  %-4d| %s\n", u.ID, u.Name)
	}
}

func cmdDelete(mgr *voiceprint.Manager, name string) {
	if err := mgr.DeleteUser(name); err != nil {
		fmt.Fprintf(os.Stderr, "删除失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("用户 %s 已删除。\n", name)
}
