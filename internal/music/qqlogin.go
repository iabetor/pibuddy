package music

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
	"golang.org/x/net/publicsuffix"
)

// QQ 音乐 OAuth 扫码登录参数（从 y.qq.com 页面分析获取）
const (
	qqMusicAppID    = "716027609"
	qqMusicDaid     = "383"
	qqMusicPt3rdAid = "100497308"
	qqMusicSURL     = "https://graph.qq.com/oauth2.0/login_jump"
)

// QRCode 二维码信息
type QRCode struct {
	ImageData []byte // PNG 图片数据
	Qrsig     string // 用于轮询的签名
}

// QRLoginResult 扫码登录结果
type QRLoginResult struct {
	Cookies []http.Cookie
	UIN     string // QQ 号
}

// GetQRCode 获取 QQ 登录二维码。
// 返回二维码 PNG 图片数据和用于轮询的 qrsig。
func GetQRCode() (*QRCode, error) {
	t := fmt.Sprintf("%.16f", rand.Float64())

	qrURL := fmt.Sprintf(
		"https://ssl.ptlogin2.qq.com/ptqrshow?appid=%s&e=2&l=M&s=3&d=72&v=4&t=%s&daid=%s&pt_3rd_aid=%s",
		qqMusicAppID, t, qqMusicDaid, qqMusicPt3rdAid,
	)

	resp, err := http.Get(qrURL)
	if err != nil {
		return nil, fmt.Errorf("获取二维码失败: %w", err)
	}
	defer resp.Body.Close()

	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取二维码图片失败: %w", err)
	}

	// 从 Set-Cookie 中提取 qrsig
	var qrsig string
	for _, c := range resp.Cookies() {
		if c.Name == "qrsig" {
			qrsig = c.Value
			break
		}
	}
	if qrsig == "" {
		return nil, fmt.Errorf("未获取到 qrsig")
	}

	return &QRCode{
		ImageData: imgData,
		Qrsig:     qrsig,
	}, nil
}

// ptqrToken 根据 qrsig 计算 ptqrtoken（QQ 登录协议要求的 hash）。
func ptqrToken(qrsig string) int {
	e := 0
	for i := 0; i < len(qrsig); i++ {
		e += (e << 5) + int(qrsig[i])
	}
	return e & 0x7FFFFFFF
}

// QRStatus 扫码状态
type QRStatus int

const (
	QRWaiting    QRStatus = iota // 等待扫码
	QRScanned                    // 已扫码，等待确认
	QRConfirmed                  // 已确认
	QRExpired                    // 已过期
	QRError                      // 错误
)

// CheckQRStatus 检查二维码扫码状态。
// 返回状态和提示信息。当状态为 QRConfirmed 时，返回重定向 URL 用于后续获取 cookie。
func CheckQRStatus(qrsig string) (QRStatus, string, error) {
	token := ptqrToken(qrsig)
	now := time.Now().UnixMilli()

	checkURL := fmt.Sprintf(
		"https://ssl.ptlogin2.qq.com/ptqrlogin?u1=%s&ptqrtoken=%d&ptredirect=0&h=1&t=1&g=1&from_ui=1&ptlang=2052&action=0-0-%d&js_ver=20032614&js_type=1&login_sig=&pt_uistyle=40&aid=%s&daid=%s&pt_3rd_aid=%s&has_signing=1",
		url.QueryEscape(qqMusicSURL), token, now, qqMusicAppID, qqMusicDaid, qqMusicPt3rdAid,
	)

	req, err := http.NewRequest("GET", checkURL, nil)
	if err != nil {
		return QRError, "", err
	}
	req.Header.Set("Cookie", "qrsig="+qrsig)
	req.Header.Set("Referer", "https://xui.ptlogin2.qq.com/")

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return QRError, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return QRError, "", err
	}

	text := string(body)

	switch {
	case strings.Contains(text, "二维码未失效"):
		return QRWaiting, "等待扫码...", nil
	case strings.Contains(text, "二维码认证中"):
		return QRScanned, "已扫码，请在手机上确认", nil
	case strings.Contains(text, "二维码已失效"):
		return QRExpired, "二维码已过期", nil
	case strings.Contains(text, "登录成功"):
		// ptuiCB 格式: ptuiCB('0','0','url','0','msg','nickname')
		re := regexp.MustCompile(`ptuiCB\('0','0','(https?://[^']+)'`)
		matches := re.FindStringSubmatch(text)
		if len(matches) < 2 {
			return QRError, "登录成功但无法提取跳转地址", nil
		}
		redirectURL := matches[1]

		// 从 URL 中提取 uin（ptlogin 回调 URL 通常包含 uin 参数）
		if u, parseErr := url.Parse(redirectURL); parseErr == nil {
			if uinVal := u.Query().Get("uin"); uinVal != "" {
				logger.Debugf("[qqmusic] 从 redirect URL 中提取到 uin: %s", uinVal)
			}
		}
		// 尝试从完整 ptuiCB 中提取 nickname
		reNick := regexp.MustCompile(`ptuiCB\('0','0','[^']*','0','[^']*','([^']*)'`)
		if nickMatches := reNick.FindStringSubmatch(text); len(nickMatches) > 1 {
			logger.Debugf("[qqmusic] 登录昵称: %s", nickMatches[1])
		}

		return QRConfirmed, redirectURL, nil
	default:
		return QRError, fmt.Sprintf("未知状态: %s", text), nil
	}
}

// gTk 根据 p_skey 计算 g_tk（QQ 登录 CSRF token）。
func gTk(pSkey string) int {
	hash := 5381
	for i := 0; i < len(pSkey); i++ {
		hash += (hash << 5) + int(pSkey[i])
	}
	return hash & 0x7FFFFFFF
}

// CompleteQQMusicLogin 完成 QQ 音乐 OAuth 登录流程。
// redirectURL 是 ptqrlogin 返回的跳转地址。
// 返回最终的 QQ 音乐 cookies。
func CompleteQQMusicLogin(redirectURL string) (*QRLoginResult, error) {
	// 从 redirectURL 中预提取 uin（作为后备）
	var uinFromURL string
	if u, err := url.Parse(redirectURL); err == nil {
		if v := u.Query().Get("uin"); v != "" {
			uinFromURL = strings.TrimLeft(strings.TrimLeft(v, "o"), "0")
		}
	}

	// 创建带 cookie jar 的 client，自动处理跨域 cookie
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 cookie jar 失败: %w", err)
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 允许自动跟随重定向，但限制次数
			if len(via) >= 15 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}

	// 第一步：访问 redirectURL，完成登录跳转，获取基础 cookie
	// 使用不自动跟随的 client 手动跟踪，以收集所有中间 cookie 和 uin
	noRedirectClient := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	currentURL := redirectURL
	for i := 0; i < 15; i++ {
		resp, err := noRedirectClient.Get(currentURL)
		if err != nil {
			break
		}
		// 从中间 URL 提取 uin
		if u, parseErr := url.Parse(currentURL); parseErr == nil {
			if v := u.Query().Get("uin"); v != "" && uinFromURL == "" {
				uinFromURL = strings.TrimLeft(strings.TrimLeft(v, "o"), "0")
			}
		}
		// 从 response cookies 提取 uin
		for _, c := range resp.Cookies() {
			if (c.Name == "uin" || c.Name == "ptui_loginuin" || c.Name == "pt2gguin") && c.Value != "" && uinFromURL == "" {
				v := strings.TrimLeft(strings.TrimLeft(c.Value, "o"), "0")
				if v != "" {
					uinFromURL = v
				}
			}
		}
		loc := resp.Header.Get("Location")
		resp.Body.Close()
		if loc == "" || resp.StatusCode < 300 || resp.StatusCode >= 400 {
			break
		}
		// 处理相对 URL
		if !strings.HasPrefix(loc, "http") {
			base, _ := url.Parse(currentURL)
			if ref, err := base.Parse(loc); err == nil {
				loc = ref.String()
			}
		}
		currentURL = loc
	}

	// 收集所有域名的 cookie
	allCookies := make(map[string]*http.Cookie)
	collectCookies := func(u *url.URL) {
		for _, c := range jar.Cookies(u) {
			allCookies[c.Name] = c
		}
	}

	// 收集各域名的 cookie
	for _, domain := range []string{
		"https://qq.com",
		"https://qq.com/",
		"https://graph.qq.com",
		"https://ssl.ptlogin2.qq.com",
		"https://y.qq.com",
	} {
		u, _ := url.Parse(domain)
		collectCookies(u)
	}

	// 查找关键 cookie
	var pSkey string
	for name, c := range allCookies {
		switch name {
		case "p_skey":
			pSkey = c.Value
		case "uin", "ptui_loginuin", "pt2gguin":
			if uinFromURL == "" {
				v := strings.TrimLeft(strings.TrimLeft(c.Value, "o"), "0")
				if v != "" {
					uinFromURL = v
				}
			}
		}
	}

	if pSkey == "" {
		// 尝试直接返回已获取的 cookie（可能不需要 OAuth）
		if len(allCookies) > 0 {
			return buildLoginResult(allCookies, uinFromURL)
		}
		return nil, fmt.Errorf("未获取到 p_skey")
	}

	// 第二步：OAuth 授权，获取 code
	gtk := gTk(pSkey)
	redirectURI := "https://y.qq.com/portal/wx_redirect.html?login_type=1&surl=https://y.qq.com/"
	authorizeURL := fmt.Sprintf(
		"https://graph.qq.com/oauth2.0/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=all&state=&g_tk=%d",
		qqMusicPt3rdAid, url.QueryEscape(redirectURI), gtk,
	)

	// 手动构建请求，带 cookie
	req, err := http.NewRequest("GET", authorizeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建授权请求失败: %w", err)
	}
	req.Header.Set("Referer", "https://y.qq.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// 手动附加 cookie（跨域时 jar 可能不会自动附加）
	for _, c := range allCookies {
		req.AddCookie(c)
	}

	// 不自动跟随，获取 code
	client2 := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client2.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OAuth 授权请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 收集响应 cookie
	for _, c := range resp.Cookies() {
		allCookies[c.Name] = c
	}

	// 从 Location 头提取 code
	var code string
	if loc := resp.Header.Get("Location"); loc != "" {
		if u, err := url.Parse(loc); err == nil {
			code = u.Query().Get("code")
		}
		// 跟随重定向收集更多 cookie
		resp2, err := client.Get(loc)
		if err == nil {
			for _, c := range resp2.Cookies() {
				allCookies[c.Name] = c
			}
			resp2.Body.Close()
		}
	}

	// 如果没有 code，尝试从响应体解析
	if code == "" {
		body, _ := io.ReadAll(resp.Body)
		// 尝试 JSONP 格式: callback({"code":"xxx"})
		re := regexp.MustCompile(`"code"\s*:\s*"([^"]+)"`)
		if matches := re.FindStringSubmatch(string(body)); len(matches) > 1 {
			code = matches[1]
		}
		// 尝试 URL 格式: code=xxx
		if code == "" {
			re = regexp.MustCompile(`code=([A-Za-z0-9_-]+)`)
			if matches := re.FindStringSubmatch(string(body)); len(matches) > 1 {
				code = matches[1]
			}
		}
	}

	// 第三步：用 code 换取 QQ 音乐 token
	if code != "" {
		musicCookies, err := exchangeQQMusicToken(client2, code, gtk, allCookies)
		if err == nil && len(musicCookies) > 0 {
			for _, c := range musicCookies {
				allCookies[c.Name] = c
			}
		}
	}

	// 最后再收集一次所有域名的 cookie
	for _, domain := range []string{
		"https://y.qq.com",
		"https://qq.com",
		"https://graph.qq.com",
	} {
		u, _ := url.Parse(domain)
		collectCookies(u)
	}

	return buildLoginResult(allCookies, uinFromURL)
}

// exchangeQQMusicToken 使用 OAuth code 换取 QQ 音乐 token。
func exchangeQQMusicToken(client *http.Client, code string, gtk int, existingCookies map[string]*http.Cookie) ([]*http.Cookie, error) {
	payload := map[string]interface{}{
		"comm": map[string]interface{}{
			"g_tk":     gtk,
			"platform": "yqq",
			"ct":       24,
			"cv":       0,
		},
		"req": map[string]interface{}{
			"module": "QQConnectLogin.LoginServer",
			"method": "QQLogin",
			"param": map[string]interface{}{
				"code": code,
			},
		},
	}

	data, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://u.y.qq.com/cgi-bin/musicu.fcg", strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://y.qq.com/")

	for _, c := range existingCookies {
		req.AddCookie(c)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return resp.Cookies(), nil
}

func buildLoginResult(cookies map[string]*http.Cookie, fallbackUIN ...string) (*QRLoginResult, error) {
	if len(cookies) == 0 {
		return nil, fmt.Errorf("未获取到任何 cookie")
	}

	var result QRLoginResult
	for _, c := range cookies {
		result.Cookies = append(result.Cookies, http.Cookie{
			Name:  c.Name,
			Value: c.Value,
		})
		// QQ 号可能在不同 cookie 字段中：uin, ptui_loginuin, pt2gguin
		if c.Name == "uin" || c.Name == "ptui_loginuin" || c.Name == "pt2gguin" {
			// 清理 uin 前缀的 "o" 字符和前导零
			uin := strings.TrimLeft(c.Value, "o")
			uin = strings.TrimLeft(uin, "0")
			if uin != "" && result.UIN == "" {
				result.UIN = uin
			}
		}
	}

	// 如果从 cookie 中没找到 uin，使用 fallback（从 redirectURL 中提取的）
	if result.UIN == "" && len(fallbackUIN) > 0 && fallbackUIN[0] != "" {
		result.UIN = fallbackUIN[0]
	}

	return &result, nil
}

// SetQQMusicAPICookie 将 cookie 同步到 QQMusicApi 服务。
func SetQQMusicAPICookie(apiURL string, cookies []http.Cookie) error {
	var parts []string
	for _, c := range cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	cookieStr := strings.Join(parts, "; ")

	payload, _ := json.Marshal(map[string]string{
		"data": cookieStr,
	})

	resp, err := http.Post(
		strings.TrimSuffix(apiURL, "/")+"/user/setCookie",
		"application/json",
		strings.NewReader(string(payload)),
	)
	if err != nil {
		return fmt.Errorf("设置 QQMusicApi cookie 失败: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
