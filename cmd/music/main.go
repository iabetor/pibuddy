package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
	"github.com/iabetor/pibuddy/internal/music"
)

const (
	defaultNeteaseURL = "http://localhost:3000"
	defaultQQURL      = "http://localhost:3300"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// 解析 provider 和 command
	provider, command, opts := parseArgs()

	// 初始化日志
	logLevel := "info"
	if opts.verbose {
		logLevel = "debug"
	}
	logger.Init(logger.Config{Level: logLevel})
	defer logger.Sync()

	dataDir := getDataDir()
	apiURL := getAPIURL(provider)

	switch command {
	case "login":
		if provider == "qq" {
			if opts.cookie != "" {
				doQQLoginWithCookie(apiURL, dataDir, opts.cookie)
			} else if opts.webMode {
				doQQLoginWeb(apiURL, dataDir, opts.port)
			} else {
				doQQLogin(apiURL, dataDir)
			}
		} else {
			doNeteaseLogin(apiURL, dataDir)
		}
	case "status":
		if provider == "qq" {
			doQQStatus(apiURL, dataDir)
		} else {
			doNeteaseStatus(apiURL, dataDir)
		}
	case "logout":
		doLogout(provider, dataDir)
	default:
		printUsage()
		os.Exit(1)
	}
}

type cmdOptions struct {
	webMode bool
	port    string
	cookie  string
	verbose bool
}

func parseArgs() (string, string, cmdOptions) {
	opts := cmdOptions{port: "8099"}

	if len(os.Args) < 2 {
		return "qq", "", opts
	}

	// 收集非 flag 参数
	var positional []string
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--web":
			opts.webMode = true
		case "-v", "--verbose":
			opts.verbose = true
		case "--port":
			if i+1 < len(os.Args) {
				i++
				opts.port = os.Args[i]
			}
		case "--cookie":
			if i+1 < len(os.Args) {
				i++
				opts.cookie = os.Args[i]
			}
		default:
			positional = append(positional, os.Args[i])
		}
	}

	if len(positional) == 0 {
		return "qq", "", opts
	}

	arg1 := positional[0]
	if arg1 == "qq" || arg1 == "netease" {
		if len(positional) < 2 {
			return arg1, "", opts
		}
		return arg1, positional[1], opts
	}

	// 默认 qq
	return "qq", arg1, opts
}

func printUsage() {
	fmt.Println("音乐登录工具")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  pibuddy-music [provider] <command> [options]")
	fmt.Println("")
	fmt.Println("Provider:")
	fmt.Println("  qq       QQ 音乐 (默认)")
	fmt.Println("  netease  网易云音乐")
	fmt.Println("")
	fmt.Println("命令:")
	fmt.Println("  login    登录")
	fmt.Println("  status   查看登录状态")
	fmt.Println("  logout   退出登录")
	fmt.Println("")
	fmt.Println("选项:")
	fmt.Println("  --web     QQ 登录时启动 Web 服务器展示二维码，方便手机扫码")
	fmt.Println("  --port    Web 服务器端口 (默认: 8099)")
	fmt.Println("  --cookie  直接导入浏览器 cookie 字符串 (格式: name1=value1; name2=value2)")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  pibuddy-music login              # 登录 QQ 音乐（终端扫码）")
	fmt.Println("  pibuddy-music login --web        # 登录 QQ 音乐（手机浏览器扫码）")
	fmt.Println("  pibuddy-music login --cookie '...'  # 导入浏览器 cookie")
	fmt.Println("  pibuddy-music status             # 查看 QQ 音乐登录状态")
	fmt.Println("  pibuddy-music netease login      # 登录网易云音乐")
	fmt.Println("")
	fmt.Println("环境变量:")
	fmt.Println("  PIBUDDY_MUSIC_API_URL    API 地址 (网易云默认: http://localhost:3000)")
	fmt.Println("  PIBUDDY_QQ_MUSIC_API_URL QQ 音乐 API 地址 (默认: http://localhost:3300)")
	fmt.Println("  PIBUDDY_DATA_DIR         数据目录 (默认: ~/.pibuddy)")
}

func getDataDir() string {
	dataDir := os.Getenv("PIBUDDY_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			dataDir = home + "/.pibuddy"
		} else {
			dataDir = "./.pibuddy-data"
		}
	}
	return dataDir
}

func getAPIURL(provider string) string {
	switch provider {
	case "qq":
		apiURL := os.Getenv("PIBUDDY_QQ_MUSIC_API_URL")
		if apiURL == "" {
			apiURL = defaultQQURL
		}
		return apiURL
	default:
		apiURL := os.Getenv("PIBUDDY_MUSIC_API_URL")
		if apiURL == "" {
			apiURL = defaultNeteaseURL
		}
		return apiURL
	}
}

func cookieFileName(provider string) string {
	switch provider {
	case "qq":
		return "qq_cookie.json"
	default:
		return "netease_cookie.json"
	}
}

// ============================================================
// QQ 音乐登录（终端扫码）
// ============================================================

func doQQLogin(apiURL, dataDir string) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建数据目录失败: %v\n", err)
		os.Exit(1)
	}

	// 检查现有登录状态
	cookiePath := filepath.Join(dataDir, "qq_cookie.json")
	if data, err := loadCookieData(cookiePath); err == nil && data.LoggedIn {
		fmt.Printf("当前已登录: %s\n", data.User)
		fmt.Print("是否重新登录? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(input)) != "y" {
			return
		}
	}

	fmt.Println("============================================")
	fmt.Println("QQ 音乐扫码登录")
	fmt.Println("============================================")
	fmt.Println()

	// 获取二维码
	fmt.Print("正在获取二维码...")
	qr, err := music.GetQRCode()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n获取二维码失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(" 完成")

	// 保存二维码图片
	qrPath := filepath.Join(dataDir, "qq_qrcode.png")
	if err := os.WriteFile(qrPath, qr.ImageData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "保存二维码失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("二维码已保存到: %s\n", qrPath)
	fmt.Println()
	fmt.Println("请用手机 QQ 扫描二维码登录：")
	fmt.Println("  1. 打开手机 QQ")
	fmt.Println("  2. 点击右上角 + → 扫一扫")
	fmt.Println("  3. 扫描上述二维码文件")
	fmt.Println()

	// 轮询扫码状态
	fmt.Println("等待扫码中...")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(120 * time.Second)
	lastStatus := music.QRWaiting

	for {
		select {
		case <-timeout:
			fmt.Println("\n✗ 超时未扫码，请重新执行登录命令")
			os.Exit(1)
		case <-ticker.C:
			status, msg, err := music.CheckQRStatus(qr.Qrsig)
			if err != nil {
				fmt.Fprintf(os.Stderr, "\n检查状态失败: %v\n", err)
				continue
			}

			switch status {
			case music.QRWaiting:
				if lastStatus != music.QRWaiting {
					fmt.Printf("  %s\n", msg)
				}
			case music.QRScanned:
				if lastStatus != music.QRScanned {
					fmt.Printf("  ✓ %s\n", msg)
				}
			case music.QRExpired:
				fmt.Printf("  ✗ %s\n", msg)
				fmt.Println("请重新执行登录命令")
				os.Exit(1)
			case music.QRConfirmed:
				fmt.Println("  ✓ 扫码成功！正在获取登录凭据...")

				// 完成 OAuth 流程
				result, err := music.CompleteQQMusicLogin(msg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "获取登录凭据失败: %v\n", err)
					os.Exit(1)
				}

				if len(result.Cookies) == 0 {
					fmt.Fprintln(os.Stderr, "未获取到 cookie")
					os.Exit(1)
				}

				uin := result.UIN
				if uin == "" {
					uin = "unknown"
				}

				// 保存 cookie
				data := cookieData{
					Cookies:   result.Cookies,
					LoggedIn:  true,
					User:      uin,
					UpdatedAt: time.Now(),
				}

				if err := saveCookieData(cookiePath, &data); err != nil {
					fmt.Fprintf(os.Stderr, "保存 cookie 失败: %v\n", err)
					os.Exit(1)
				}

				fmt.Println()
				fmt.Printf("✓ 登录成功！QQ 号: %s\n", uin)
				fmt.Printf("✓ Cookie 已保存到: %s (%d 个)\n", cookiePath, len(result.Cookies))

				// 同步到 QQMusicApi
				if apiURL != "" {
					fmt.Printf("  正在同步到 QQMusicApi (%s)...", apiURL)
					if err := music.SetQQMusicAPICookie(apiURL, result.Cookies); err != nil {
						fmt.Printf(" 跳过 (%v)\n", err)
					} else {
						fmt.Println(" 完成")
					}
				}

				// 清理二维码
				os.Remove(qrPath)
				return

			case music.QRError:
				fmt.Fprintf(os.Stderr, "  ✗ 错误: %s\n", msg)
				os.Exit(1)
			}
			lastStatus = status
		}
	}
}

// ============================================================
// QQ 音乐登录（导入浏览器 cookie）
// ============================================================

func doQQLoginWithCookie(apiURL, dataDir, cookieStr string) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建数据目录失败: %v\n", err)
		os.Exit(1)
	}

	cookiePath := filepath.Join(dataDir, "qq_cookie.json")

	fmt.Println("============================================")
	fmt.Println("QQ 音乐 Cookie 导入")
	fmt.Println("============================================")
	fmt.Println()

	// 解析 cookie 字符串
	var cookies []http.Cookie
	for _, part := range strings.Split(cookieStr, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.Index(part, "=")
		if idx < 1 {
			continue
		}
		name := strings.TrimSpace(part[:idx])
		value := strings.TrimSpace(part[idx+1:])
		cookies = append(cookies, http.Cookie{
			Name:  name,
			Value: value,
		})
	}

	if len(cookies) == 0 {
		fmt.Fprintln(os.Stderr, "✗ 未解析到有效的 cookie")
		os.Exit(1)
	}

	// 提取 UIN
	var uin string
	for _, c := range cookies {
		if c.Name == "uin" || c.Name == "ptui_loginuin" || c.Name == "pt2gguin" {
			uin = strings.TrimLeft(strings.TrimLeft(c.Value, "o"), "0")
			break
		}
	}
	if uin == "" {
		uin = "unknown"
	}

	// 检查关键 cookie
	var hasQQMusicKey, hasQmKeyst bool
	for _, c := range cookies {
		if c.Name == "qqmusic_key" && c.Value != "" {
			hasQQMusicKey = true
		}
		if c.Name == "qm_keyst" && c.Value != "" {
			hasQmKeyst = true
		}
	}

	// 保存 cookie
	data := cookieData{
		Cookies:   cookies,
		LoggedIn:  true,
		User:      uin,
		UpdatedAt: time.Now(),
	}

	if err := saveCookieData(cookiePath, &data); err != nil {
		fmt.Fprintf(os.Stderr, "保存 cookie 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ 导入成功！QQ 号: %s\n", uin)
	fmt.Printf("✓ Cookie 已保存到: %s (%d 个)\n", cookiePath, len(cookies))

	// 检查关键字段
	fmt.Println()
	if hasQQMusicKey && hasQmKeyst {
		fmt.Println("✓ 关键 cookie 完整 (qqmusic_key, qm_keyst)")
	} else {
		fmt.Println("⚠ 缺少关键 cookie:")
		if !hasQQMusicKey {
			fmt.Println("  - qqmusic_key (VIP 播放需要)")
		}
		if !hasQmKeyst {
			fmt.Println("  - qm_keyst")
		}
		fmt.Println()
		fmt.Println("请在浏览器登录 y.qq.com 后重新复制完整 cookie")
	}

	// 同步到 QQMusicApi
	if apiURL != "" {
		fmt.Println()
		fmt.Printf("正在同步到 QQMusicApi (%s)...", apiURL)
		if err := music.SetQQMusicAPICookie(apiURL, cookies); err != nil {
			fmt.Printf(" 失败: %v\n", err)
		} else {
			fmt.Println(" 完成")
		}
	}
}

// ============================================================
// QQ 音乐 Web 扫码登录（手机浏览器访问）
// ============================================================

func doQQLoginWeb(apiURL, dataDir, port string) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建数据目录失败: %v\n", err)
		os.Exit(1)
	}

	cookiePath := filepath.Join(dataDir, "qq_cookie.json")

	// 登录状态管理
	var (
		mu         sync.Mutex
		qr         *music.QRCode
		qrB64      string
		statusText = "initializing"
		loginDone  = make(chan struct{})
	)

	// 获取本机 IP
	localIP := getLocalIP()

	mux := http.NewServeMux()

	// 主页面：展示二维码 + 实时状态
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		img := qrB64
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, qrLoginHTML, img)
	})

	// SSE：推送扫码状态
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		lastSent := ""
		for {
			select {
			case <-r.Context().Done():
				return
			case <-loginDone:
				mu.Lock()
				s := statusText
				mu.Unlock()
				fmt.Fprintf(w, "data: %s\n\n", s)
				flusher.Flush()
				return
			case <-time.After(500 * time.Millisecond):
				mu.Lock()
				s := statusText
				mu.Unlock()
				if s != lastSent {
					fmt.Fprintf(w, "data: %s\n\n", s)
					flusher.Flush()
					lastSent = s
				}
			}
		}
	})

	// 刷新二维码
	mux.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		newQR, err := music.GetQRCode()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		mu.Lock()
		qr = newQR
		qrB64 = base64.StdEncoding.EncodeToString(qr.ImageData)
		statusText = "waiting"
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"qr":"%s"}`, qrB64)
	})

	// 启动 HTTP 服务器
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "启动 Web 服务器失败: %v\n", err)
		os.Exit(1)
	}
	server := &http.Server{Handler: mux}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Web 服务器异常: %v\n", err)
		}
	}()

	fmt.Println("============================================")
	fmt.Println("QQ 音乐 Web 扫码登录")
	fmt.Println("============================================")
	fmt.Println()

	// 获取二维码
	qr, err = music.GetQRCode()
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取二维码失败: %v\n", err)
		os.Exit(1)
	}

	mu.Lock()
	qrB64 = base64.StdEncoding.EncodeToString(qr.ImageData)
	statusText = "waiting"
	mu.Unlock()

	fmt.Printf("请在手机浏览器打开:\n\n")
	fmt.Printf("  http://%s:%s\n\n", localIP, port)
	fmt.Println("然后用手机 QQ 扫描页面上的二维码完成登录。")
	fmt.Println()
	fmt.Println("等待扫码中...")

	// 轮询扫码状态
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(180 * time.Second)
	lastStatus := music.QRWaiting

	for {
		select {
		case <-timeout:
			mu.Lock()
			statusText = "expired"
			mu.Unlock()
			fmt.Println("\n✗ 超时未扫码")
			server.Close()
			os.Exit(1)

		case <-ticker.C:
			mu.Lock()
			currentQR := qr
			mu.Unlock()

			if currentQR == nil {
				continue
			}

			status, msg, err := music.CheckQRStatus(currentQR.Qrsig)
			if err != nil {
				continue
			}

			switch status {
			case music.QRWaiting:
				mu.Lock()
				statusText = "waiting"
				mu.Unlock()

			case music.QRScanned:
				if lastStatus != music.QRScanned {
					fmt.Println("  ✓ 已扫码，等待确认...")
				}
				mu.Lock()
				statusText = "scanned"
				mu.Unlock()

			case music.QRExpired:
				fmt.Println("  ✗ 二维码已过期，请在页面上点击刷新")
				mu.Lock()
				statusText = "expired"
				mu.Unlock()

			case music.QRConfirmed:
				fmt.Println("  ✓ 扫码成功！正在获取登录凭据...")
				mu.Lock()
				statusText = "logging_in"
				mu.Unlock()

				result, err := music.CompleteQQMusicLogin(msg)
				if err != nil {
					mu.Lock()
					statusText = "error:" + err.Error()
					mu.Unlock()
					fmt.Fprintf(os.Stderr, "获取登录凭据失败: %v\n", err)
					server.Close()
					os.Exit(1)
				}

				if len(result.Cookies) == 0 {
					mu.Lock()
					statusText = "error:未获取到 cookie"
					mu.Unlock()
					fmt.Fprintln(os.Stderr, "未获取到 cookie")
					server.Close()
					os.Exit(1)
				}

				uin := result.UIN
				if uin == "" {
					uin = "unknown"
				}

				data := cookieData{
					Cookies:   result.Cookies,
					LoggedIn:  true,
					User:      uin,
					UpdatedAt: time.Now(),
				}

				if err := saveCookieData(cookiePath, &data); err != nil {
					fmt.Fprintf(os.Stderr, "保存 cookie 失败: %v\n", err)
				}

				// 同步到 QQMusicApi
				if apiURL != "" {
					if err := music.SetQQMusicAPICookie(apiURL, result.Cookies); err != nil {
						fmt.Printf("  同步到 QQMusicApi 跳过: %v\n", err)
					} else {
						fmt.Println("  ✓ 已同步到 QQMusicApi")
					}
				}

				mu.Lock()
				statusText = "success:" + uin
				mu.Unlock()
				close(loginDone)

				fmt.Println()
				fmt.Printf("✓ 登录成功！QQ 号: %s\n", uin)
				fmt.Printf("✓ Cookie 已保存 (%d 个)\n", len(result.Cookies))

				// 等一小会让 SSE 推送完成
				time.Sleep(2 * time.Second)
				server.Close()
				return

			case music.QRError:
				mu.Lock()
				statusText = "error:" + msg
				mu.Unlock()
				fmt.Fprintf(os.Stderr, "  ✗ 错误: %s\n", msg)
				server.Close()
				os.Exit(1)
			}
			lastStatus = status
		}
	}
}

// getLocalIP 获取本机局域网 IP。
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "localhost"
}

// qrLoginHTML 是二维码登录页面的 HTML 模板。
const qrLoginHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
<title>QQ音乐登录 - PiBuddy</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 20px;
  }
  .card {
    background: white;
    border-radius: 20px;
    padding: 40px 30px;
    text-align: center;
    box-shadow: 0 20px 60px rgba(0,0,0,0.3);
    max-width: 380px;
    width: 100%%;
  }
  h1 {
    font-size: 22px;
    color: #333;
    margin-bottom: 8px;
  }
  .subtitle {
    font-size: 14px;
    color: #999;
    margin-bottom: 24px;
  }
  .qr-container {
    position: relative;
    display: inline-block;
    margin-bottom: 20px;
  }
  .qr-container img {
    width: 240px;
    height: 240px;
    border-radius: 12px;
    border: 3px solid #eee;
  }
  .qr-overlay {
    display: none;
    position: absolute;
    top: 0; left: 0; right: 0; bottom: 0;
    background: rgba(255,255,255,0.95);
    border-radius: 12px;
    align-items: center;
    justify-content: center;
    flex-direction: column;
    cursor: pointer;
  }
  .qr-overlay.show { display: flex; }
  .qr-overlay .icon { font-size: 40px; margin-bottom: 8px; }
  .qr-overlay .text { font-size: 14px; color: #666; }
  .status {
    font-size: 16px;
    padding: 12px 20px;
    border-radius: 10px;
    margin-bottom: 16px;
    transition: all 0.3s;
  }
  .status.waiting { background: #f0f4ff; color: #5b7ce5; }
  .status.scanned { background: #f0fff4; color: #38a169; }
  .status.success { background: #f0fff4; color: #22863a; }
  .status.error   { background: #fff5f5; color: #e53e3e; }
  .status.expired { background: #fffaf0; color: #dd6b20; }
  .steps {
    text-align: left;
    font-size: 13px;
    color: #888;
    line-height: 2;
    padding-left: 10px;
  }
  .steps b { color: #555; }
  .refresh-btn {
    display: none;
    margin: 12px auto;
    padding: 10px 24px;
    border: none;
    border-radius: 8px;
    background: #667eea;
    color: white;
    font-size: 14px;
    cursor: pointer;
  }
  .refresh-btn:active { background: #5a6fd6; }
</style>
</head>
<body>
<div class="card">
  <h1>QQ音乐扫码登录</h1>
  <p class="subtitle">PiBuddy 音乐服务</p>
  <div class="qr-container">
    <img id="qr" src="data:image/png;base64,%s" alt="QR Code">
    <div class="qr-overlay" id="overlay" onclick="refreshQR()">
      <span class="icon">🔄</span>
      <span class="text">点击刷新二维码</span>
    </div>
  </div>
  <div class="status waiting" id="status">等待扫码...</div>
  <button class="refresh-btn" id="refreshBtn" onclick="refreshQR()">刷新二维码</button>
  <div class="steps" id="steps">
    <b>步骤：</b><br>
    1. 长按保存上方二维码<br>
    2. 打开手机QQ → 右上角 + → 扫一扫<br>
    3. 选择相册中的二维码图片<br>
    4. 在QQ中确认登录
  </div>
</div>
<script>
const statusEl = document.getElementById('status');
const overlayEl = document.getElementById('overlay');
const refreshBtn = document.getElementById('refreshBtn');
const stepsEl = document.getElementById('steps');

function refreshQR() {
  overlayEl.classList.remove('show');
  refreshBtn.style.display = 'none';
  statusEl.textContent = '正在刷新...';
  statusEl.className = 'status waiting';
  fetch('/refresh')
    .then(r => r.json())
    .then(d => {
      if (d.success) {
        document.getElementById('qr').src = 'data:image/png;base64,' + d.qr;
        statusEl.textContent = '等待扫码...';
      }
    })
    .catch(() => { statusEl.textContent = '刷新失败，请重试'; });
}

const evtSource = new EventSource('/status');
evtSource.onmessage = function(e) {
  const s = e.data;
  if (s === 'waiting') {
    statusEl.textContent = '等待扫码...';
    statusEl.className = 'status waiting';
    overlayEl.classList.remove('show');
    refreshBtn.style.display = 'none';
  } else if (s === 'scanned') {
    statusEl.textContent = '✓ 已扫码，请在QQ中点击确认';
    statusEl.className = 'status scanned';
  } else if (s === 'expired') {
    statusEl.textContent = '二维码已过期';
    statusEl.className = 'status expired';
    overlayEl.classList.add('show');
    refreshBtn.style.display = 'block';
  } else if (s === 'logging_in') {
    statusEl.textContent = '正在获取登录凭据...';
    statusEl.className = 'status scanned';
  } else if (s.startsWith('success:')) {
    const uin = s.substring(8);
    statusEl.textContent = '✓ 登录成功！QQ号: ' + uin;
    statusEl.className = 'status success';
    stepsEl.innerHTML = '<b>登录完成，可以关闭此页面了</b>';
    evtSource.close();
  } else if (s.startsWith('error:')) {
    statusEl.textContent = '✗ ' + s.substring(6);
    statusEl.className = 'status error';
    evtSource.close();
  }
};
evtSource.onerror = function() {
  if (statusEl.className.indexOf('success') === -1) {
    statusEl.textContent = '连接已断开';
    statusEl.className = 'status error';
  }
};
</script>
</body>
</html>`

func doQQStatus(apiURL, dataDir string) {
	cookiePath := filepath.Join(dataDir, "qq_cookie.json")
	data, err := loadCookieData(cookiePath)

	fmt.Println("============================================")
	fmt.Println("QQ 音乐登录状态")
	fmt.Println("============================================")
	fmt.Println()

	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("状态: 未登录（无 cookie 文件）")
			fmt.Println()
			fmt.Println("运行以下命令登录:")
			fmt.Println("  pibuddy-music qq login")
		} else {
			fmt.Fprintf(os.Stderr, "读取 cookie 文件失败: %v\n", err)
		}
		os.Exit(1)
	}

	fmt.Printf("QQ 号: %s\n", data.User)
	fmt.Printf("更新时间: %s\n", data.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Cookie 数量: %d\n", len(data.Cookies))
	fmt.Println()

	// 检查关键 cookie 是否存在
	hasCookies := map[string]bool{"uin": false, "qm_keyst": false, "qqmusic_key": false}
	for _, c := range data.Cookies {
		if _, ok := hasCookies[c.Name]; ok {
			hasCookies[c.Name] = true
		}
	}

	allPresent := true
	for name, present := range hasCookies {
		if present {
			fmt.Printf("  ✓ %s\n", name)
		} else {
			fmt.Printf("  ✗ %s (缺失)\n", name)
			allPresent = false
		}
	}

	fmt.Println()
	if allPresent {
		fmt.Println("✓ 关键 cookie 完整")
	} else {
		fmt.Println("⚠ 部分关键 cookie 缺失，可能需要重新登录")
		fmt.Println("  pibuddy-music qq login")
	}

	// 尝试通过 QQMusicApi 验证
	if apiURL != "" {
		fmt.Println()
		fmt.Printf("正在通过 QQMusicApi (%s) 验证...\n", apiURL)
		cookieStr := cookieString(data.Cookies)
		req, err := http.NewRequest("GET", strings.TrimSuffix(apiURL, "/")+"/user/cookie", nil)
		if err == nil {
			req.Header.Set("Cookie", cookieStr)
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				fmt.Printf("  API 响应: %d\n", resp.StatusCode)
			} else {
				fmt.Printf("  API 连接失败: %v\n", err)
			}
		}
	}
}

// ============================================================
// 网易云音乐登录（原有逻辑）
// ============================================================

type loginStatus struct {
	Code    int `json:"code"`
	Account struct {
		ID       int64  `json:"id"`
		UserName string `json:"userName"`
		NickName string `json:"nickname"`
	} `json:"account"`
	Profile struct {
		NickName string `json:"nickname"`
	} `json:"profile"`
}

type cookieData struct {
	Cookies   []http.Cookie `json:"cookies"`
	LoggedIn  bool          `json:"logged_in"`
	User      string        `json:"user"`
	UpdatedAt time.Time     `json:"updated_at"`
}

func doNeteaseLogin(apiURL, dataDir string) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建数据目录失败: %v\n", err)
		os.Exit(1)
	}

	cookiePath := filepath.Join(dataDir, "netease_cookie.json")

	// 检查当前登录状态
	if status := checkLoginStatus(apiURL, nil); status != nil && status.Code == 200 {
		fmt.Printf("已登录用户: %s\n", getDisplayName(status))
		fmt.Print("是否重新登录? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(input)) != "y" {
			return
		}
	}

	loginURL := apiURL + "/#/login"
	fmt.Println("============================================")
	fmt.Println("网易云音乐登录")
	fmt.Println("============================================")
	fmt.Println()
	fmt.Printf("请在浏览器打开以下地址登录:\n\n%s\n\n", loginURL)
	fmt.Println("登录成功后，按回车继续...")
	fmt.Print("> ")

	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')

	// 获取登录状态
	status := checkLoginStatus(apiURL, nil)
	if status == nil || status.Code != 200 {
		fmt.Println("✗ 未检测到登录状态，请确保已在浏览器中完成登录")
		os.Exit(1)
	}

	// 获取 cookie
	cookies := fetchCookies(apiURL)
	if len(cookies) == 0 {
		fmt.Fprintf(os.Stderr, "获取 cookie 失败\n")
		os.Exit(1)
	}

	// 保存 cookie
	data := cookieData{
		Cookies:   cookies,
		LoggedIn:  true,
		User:      getDisplayName(status),
		UpdatedAt: time.Now(),
	}

	if err := saveCookieData(cookiePath, &data); err != nil {
		fmt.Fprintf(os.Stderr, "保存 cookie 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ 登录成功！用户: %s\n", data.User)
	fmt.Printf("✓ Cookie 已保存到: %s\n", cookiePath)
}

func doNeteaseStatus(apiURL, dataDir string) {
	cookiePath := filepath.Join(dataDir, "netease_cookie.json")
	data, err := loadCookieData(cookiePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("状态: 未登录（无 cookie 文件）")
		} else {
			fmt.Fprintf(os.Stderr, "读取 cookie 文件失败: %v\n", err)
		}
		os.Exit(1)
	}

	status := checkLoginStatus(apiURL, data.Cookies)

	fmt.Println("============================================")
	fmt.Println("网易云音乐登录状态")
	fmt.Println("============================================")
	fmt.Println()

	if data.LoggedIn && data.User != "" {
		fmt.Printf("本地记录: %s\n", data.User)
		fmt.Printf("更新时间: %s\n", data.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	fmt.Println()

	if status != nil && status.Code == 200 {
		fmt.Printf("API 状态: 已登录 (%s)\n", getDisplayName(status))
		fmt.Println()
		fmt.Println("✓ Cookie 有效")
	} else {
		fmt.Println("API 状态: 未登录")
		fmt.Println()
		fmt.Println("✗ Cookie 已过期，请重新登录: pibuddy-music login")
	}
}

// ============================================================
// 公共工具函数
// ============================================================

func doLogout(provider, dataDir string) {
	cookiePath := filepath.Join(dataDir, cookieFileName(provider))
	if err := os.Remove(cookiePath); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("已处于未登录状态")
		} else {
			fmt.Fprintf(os.Stderr, "删除 cookie 失败: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("✓ 已退出登录")
	}
}

func checkLoginStatus(apiURL string, cookies []http.Cookie) *loginStatus {
	req, err := http.NewRequest("GET", apiURL+"/login/status", nil)
	if err != nil {
		return nil
	}
	for _, c := range cookies {
		req.AddCookie(&c)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var status loginStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil
	}
	return &status
}

func fetchCookies(apiURL string) []http.Cookie {
	client := &http.Client{Timeout: 10 * time.Second}

	var allCookies []http.Cookie
	seen := make(map[string]bool)

	endpoints := []string{"/user/account", "/login/status"}
	for _, ep := range endpoints {
		req, _ := http.NewRequest("GET", apiURL+ep, nil)
		for _, c := range allCookies {
			req.AddCookie(&c)
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		for _, c := range resp.Cookies() {
			if !seen[c.Name] {
				seen[c.Name] = true
				allCookies = append(allCookies, *c)
			}
		}
		resp.Body.Close()
	}

	return allCookies
}

func saveCookieData(path string, data *cookieData) error {
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0600)
}

func loadCookieData(path string) (*cookieData, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data cookieData
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func getDisplayName(status *loginStatus) string {
	if status.Profile.NickName != "" {
		return status.Profile.NickName
	}
	if status.Account.NickName != "" {
		return status.Account.NickName
	}
	if status.Account.UserName != "" {
		return status.Account.UserName
	}
	return fmt.Sprintf("用户%d", status.Account.ID)
}

func cookieString(cookies []http.Cookie) string {
	var parts []string
	for _, c := range cookies {
		parts = append(parts, url.QueryEscape(c.Name)+"="+url.QueryEscape(c.Value))
	}
	return strings.Join(parts, "; ")
}
