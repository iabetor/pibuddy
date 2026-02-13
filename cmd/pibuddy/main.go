package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/iabetor/pibuddy/internal/config"
	"github.com/iabetor/pibuddy/internal/pipeline"
)

func main() {
	configPath := flag.String("config", "configs/pibuddy.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	log.Printf("[main] PiBuddy 启动中 (log_level=%s)", cfg.Log.Level)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听系统信号，优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("[main] 收到信号 %v，正在关闭...", sig)
		cancel()
	}()

	p, err := pipeline.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建流水线失败: %v\n", err)
		os.Exit(1)
	}
	defer p.Close()

	if err := p.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "流水线运行出错: %v\n", err)
		os.Exit(1)
	}

	log.Println("[main] PiBuddy 已停止")
}
