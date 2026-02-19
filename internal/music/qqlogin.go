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
		// 提取重定向 URL
		re := regexp.MustCompile(`ptuiCB\('0','0','(https?://[^']+)'`)
		matches := re.FindStringSubmatch(text)
		if len(matches) < 2 {
			return QRError, "登录成功但无法提取跳转地址", nil
		}
		return QRConfirmed, matches[1], nil
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
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 第一步：访问 ptqrlogin 返回的跳转地址，获取 p_skey 等 cookie
	resp, err := client.Get(redirectURL)
	if err != nil {
		return nil, fmt.Errorf("访问登录跳转地址失败: %w", err)
	}
	resp.Body.Close()

	// 收集所有 cookie
	allCookies := make(map[string]*http.Cookie)
	for _, c := range resp.Cookies() {
		allCookies[c.Name] = c
	}

	// 查找 p_skey
	var pSkey string
	for _, c := range resp.Cookies() {
		if c.Name == "p_skey" {
			pSkey = c.Value
			break
		}
	}

	// 如果有 Location 头，继续跟随重定向收集 cookie
	if loc := resp.Header.Get("Location"); loc != "" {
		resp2, err := client.Get(loc)
		if err == nil {
			for _, c := range resp2.Cookies() {
				allCookies[c.Name] = c
				if c.Name == "p_skey" {
					pSkey = c.Value
				}
			}
			// 继续跟随
			if loc2 := resp2.Header.Get("Location"); loc2 != "" {
				resp3, err := client.Get(loc2)
				if err == nil {
					for _, c := range resp3.Cookies() {
						allCookies[c.Name] = c
						if c.Name == "p_skey" {
							pSkey = c.Value
						}
					}
					resp3.Body.Close()
				}
			}
			resp2.Body.Close()
		}
	}

	if pSkey == "" {
		// 没有 p_skey，尝试从 jar 中获取
		for _, domain := range []string{"https://graph.qq.com", "https://qq.com", "https://ptlogin2.qq.com"} {
			u, _ := url.Parse(domain)
			for _, c := range jar.Cookies(u) {
				if c.Name == "p_skey" {
					pSkey = c.Value
				}
				allCookies[c.Name] = &http.Cookie{Name: c.Name, Value: c.Value}
			}
		}
	}

	if pSkey == "" {
		// 没有通过 OAuth 获取 p_skey，直接用已有 cookie 尝试
		return buildLoginResult(allCookies)
	}

	// 第二步：OAuth 授权，获取 code
	gtk := gTk(pSkey)
	authorizeURL := fmt.Sprintf(
		"https://graph.qq.com/oauth2.0/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=all&state=&g_tk=%d",
		qqMusicPt3rdAid, url.QueryEscape("https://y.qq.com/portal/wx_redirect.html?login_type=1&surl=https://y.qq.com/"), gtk,
	)

	req, err := http.NewRequest("GET", authorizeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建授权请求失败: %w", err)
	}

	// 附加已有 cookie
	for _, c := range allCookies {
		req.AddCookie(c)
	}

	resp, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OAuth 授权请求失败: %w", err)
	}
	defer resp.Body.Close()

	for _, c := range resp.Cookies() {
		allCookies[c.Name] = c
	}

	// 获取重定向中的 code
	var code string
	if loc := resp.Header.Get("Location"); loc != "" {
		u, err := url.Parse(loc)
		if err == nil {
			code = u.Query().Get("code")
		}

		// 跟随重定向收集 cookie
		resp2, err := client.Get(loc)
		if err == nil {
			for _, c := range resp2.Cookies() {
				allCookies[c.Name] = c
			}
			resp2.Body.Close()
		}
	}

	if code == "" {
		// 尝试从响应体解析
		body, _ := io.ReadAll(resp.Body)
		re := regexp.MustCompile(`code=([A-Za-z0-9]+)`)
		if matches := re.FindStringSubmatch(string(body)); len(matches) > 1 {
			code = matches[1]
		}
	}

	if code != "" {
		// 第三步：用 code 换取 QQ 音乐 token
		musicCookies, err := exchangeQQMusicToken(client, code, gtk, allCookies)
		if err == nil && len(musicCookies) > 0 {
			for _, c := range musicCookies {
				allCookies[c.Name] = c
			}
		}
	}

	return buildLoginResult(allCookies)
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

func buildLoginResult(cookies map[string]*http.Cookie) (*QRLoginResult, error) {
	if len(cookies) == 0 {
		return nil, fmt.Errorf("未获取到任何 cookie")
	}

	var result QRLoginResult
	for _, c := range cookies {
		result.Cookies = append(result.Cookies, http.Cookie{
			Name:  c.Name,
			Value: c.Value,
		})
		if c.Name == "uin" || c.Name == "ptui_loginuin" {
			// 清理 uin 前缀的 "o" 字符
			uin := strings.TrimLeft(c.Value, "o")
			if uin != "" {
				result.UIN = uin
			}
		}
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
