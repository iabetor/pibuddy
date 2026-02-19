package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultAPIURL = "http://localhost:3000"
	cookieFile    = "netease_cookie.json"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	dataDir := getDataDir()
	apiURL := getAPIURL()

	switch os.Args[1] {
	case "login":
		doLogin(apiURL, dataDir)
	case "status":
		doStatus(apiURL, dataDir)
	case "logout":
		doLogout(dataDir)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("网易云音乐登录工具")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  pibuddy-music login   登录网易云音乐")
	fmt.Println("  pibuddy-music status  查看登录状态")
	fmt.Println("  pibuddy-music logout  退出登录")
	fmt.Println("")
	fmt.Println("环境变量:")
	fmt.Println("  PIBUDDY_MUSIC_API_URL  NeteaseCloudMusicApi 地址 (默认: http://localhost:3000)")
	fmt.Println("  PIBUDDY_DATA_DIR       数据目录 (默认: ~/.pibuddy)")
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

func getAPIURL() string {
	apiURL := os.Getenv("PIBUDDY_MUSIC_API_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	return apiURL
}

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

func doLogin(apiURL, dataDir string) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建数据目录失败: %v\n", err)
		os.Exit(1)
	}

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

	if err := saveCookieData(filepath.Join(dataDir, cookieFile), &data); err != nil {
		fmt.Fprintf(os.Stderr, "保存 cookie 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ 登录成功！用户: %s\n", data.User)
	fmt.Printf("✓ Cookie 已保存到: %s\n", filepath.Join(dataDir, cookieFile))
}

func doStatus(apiURL, dataDir string) {
	cookiePath := filepath.Join(dataDir, cookieFile)
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

func doLogout(dataDir string) {
	cookiePath := filepath.Join(dataDir, cookieFile)
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

	// 收集所有 cookie
	var allCookies []http.Cookie
	seen := make(map[string]bool)

	// 访问多个接口收集 cookie
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

// CookieString 将 cookie 转换为 Cookie 请求头字符串（供外部使用）
func CookieString(cookies []http.Cookie) string {
	var parts []string
	for _, c := range cookies {
		parts = append(parts, url.QueryEscape(c.Name)+"="+url.QueryEscape(c.Value))
	}
	return strings.Join(parts, "; ")
}
