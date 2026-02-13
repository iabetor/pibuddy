package tools

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

// WeatherConfig 和风天气 API 配置。
type WeatherConfig struct {
	APIKey  string
	APIHost string
	// JWT 认证（推荐）
	CredentialID   string // 凭据 ID（kid）
	ProjectID      string // 项目 ID（sub）
	PrivateKeyPath string // Ed25519 私钥文件路径
}

// WeatherTool 查询天气信息。
type WeatherTool struct {
	apiKey  string
	apiHost string
	client  *http.Client

	// JWT 认证
	useJWT       bool
	credentialID string
	projectID    string
	privateKey   ed25519.PrivateKey

	// JWT token 缓存
	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

func NewWeatherTool(cfg WeatherConfig) *WeatherTool {
	host := cfg.APIHost
	if host == "" {
		host = "devapi.qweather.com"
	}
	t := &WeatherTool{
		apiKey:  cfg.APIKey,
		apiHost: host,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// 如果提供了 JWT 配置，加载私钥
	if cfg.CredentialID != "" && cfg.ProjectID != "" && cfg.PrivateKeyPath != "" {
		privKey, err := loadEd25519PrivateKey(cfg.PrivateKeyPath)
		if err != nil {
			log.Printf("[tools] 加载 Ed25519 私钥失败: %v, 回退到 API Key 认证", err)
		} else {
			t.useJWT = true
			t.credentialID = cfg.CredentialID
			t.projectID = cfg.ProjectID
			t.privateKey = privKey
			log.Printf("[tools] 天气 API 使用 JWT 认证 (credential=%s)", cfg.CredentialID)
		}
	}

	return t
}

// loadEd25519PrivateKey 从 PEM 文件加载 Ed25519 私钥。
func loadEd25519PrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取私钥文件失败: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("PEM 解码失败")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}
	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("不是 Ed25519 私钥")
	}
	return edKey, nil
}

// base64URLEncode 执行不带 padding 的 Base64URL 编码。
func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// generateJWT 生成和风天气 JWT token。
// Header: {"alg":"EdDSA","kid":"<credentialID>"}
// Payload: {"sub":"<projectID>","iat":<now-30>,"exp":<now+3600>}
func (t *WeatherTool) generateJWT() (string, error) {
	now := time.Now().Unix()
	header := fmt.Sprintf(`{"alg":"EdDSA","kid":"%s"}`, t.credentialID)
	payload := fmt.Sprintf(`{"sub":"%s","iat":%d,"exp":%d}`, t.projectID, now-30, now+3600)

	headerB64 := base64URLEncode([]byte(header))
	payloadB64 := base64URLEncode([]byte(payload))

	signingInput := headerB64 + "." + payloadB64
	sig := ed25519.Sign(t.privateKey, []byte(signingInput))

	return signingInput + "." + base64URLEncode(sig), nil
}

// getToken 获取 JWT token，使用缓存避免每次请求都重新签名。
// token 有效期 1 小时，提前 5 分钟刷新。
func (t *WeatherTool) getToken() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cachedToken != "" && time.Now().Before(t.tokenExpiry) {
		return t.cachedToken, nil
	}

	token, err := t.generateJWT()
	if err != nil {
		return "", err
	}

	t.cachedToken = token
	t.tokenExpiry = time.Now().Add(55 * time.Minute) // 提前 5 分钟刷新
	return token, nil
}

func (t *WeatherTool) Name() string { return "get_weather" }

func (t *WeatherTool) Description() string {
	return "查询指定城市的实时天气和未来3天天气预报。当用户询问天气相关问题时使用。"
}

func (t *WeatherTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"city": {
				"type": "string",
				"description": "城市名称，例如 北京、上海、武汉"
			}
		},
		"required": ["city"]
	}`)
}

type weatherArgs struct {
	City string `json:"city"`
}

// qweatherGeoResp 和风天气城市搜索响应。
type qweatherGeoResp struct {
	Code     string `json:"code"`
	Location []struct {
		Name    string `json:"name"`
		ID      string `json:"id"`
		Adm1    string `json:"adm1"`
		Adm2    string `json:"adm2"`
		Country string `json:"country"`
	} `json:"location"`
}

// qweatherNowResp 实时天气响应。
type qweatherNowResp struct {
	Code string `json:"code"`
	Now  struct {
		ObsTime   string `json:"obsTime"`
		Temp      string `json:"temp"`
		FeelsLike string `json:"feelsLike"`
		Text      string `json:"text"`
		WindDir   string `json:"windDir"`
		WindScale string `json:"windScale"`
		Humidity  string `json:"humidity"`
		Vis       string `json:"vis"`
	} `json:"now"`
}

// qweatherForecastResp 天气预报响应。
type qweatherForecastResp struct {
	Code  string `json:"code"`
	Daily []struct {
		FxDate    string `json:"fxDate"`
		TempMax   string `json:"tempMax"`
		TempMin   string `json:"tempMin"`
		TextDay   string `json:"textDay"`
		TextNight string `json:"textNight"`
		WindDirDay   string `json:"windDirDay"`
		WindScaleDay string `json:"windScaleDay"`
	} `json:"daily"`
}

func (t *WeatherTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a weatherArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	if a.City == "" {
		return "", fmt.Errorf("城市名称不能为空")
	}

	// 1. 查询城市 ID
	locationID, cityName, err := t.lookupCity(ctx, a.City)
	if err != nil {
		return "", err
	}

	// 2. 并行查询实时天气和预报
	type nowResult struct {
		data string
		err  error
	}
	type forecastResult struct {
		data string
		err  error
	}

	nowCh := make(chan nowResult, 1)
	fcCh := make(chan forecastResult, 1)

	go func() {
		data, err := t.getNow(ctx, locationID)
		nowCh <- nowResult{data, err}
	}()
	go func() {
		data, err := t.getForecast(ctx, locationID)
		fcCh <- forecastResult{data, err}
	}()

	nr := <-nowCh
	fr := <-fcCh

	if nr.err != nil {
		return "", nr.err
	}
	if fr.err != nil {
		return "", fr.err
	}

	return fmt.Sprintf("%s天气:\n%s\n%s", cityName, nr.data, fr.data), nil
}

func (t *WeatherTool) lookupCity(ctx context.Context, city string) (string, string, error) {
	u := fmt.Sprintf("https://%s/geo/v2/city/lookup?location=%s&number=1",
		t.geoHost(), url.QueryEscape(city))

	body, err := t.doGet(ctx, u)
	if err != nil {
		return "", "", fmt.Errorf("城市查询失败: %w", err)
	}

	var resp qweatherGeoResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", "", fmt.Errorf("解析城市数据失败: %w", err)
	}

	if resp.Code != "200" || len(resp.Location) == 0 {
		return "", "", fmt.Errorf("未找到城市: %s (code=%s)", city, resp.Code)
	}

	loc := resp.Location[0]
	log.Printf("[tools] 天气查询城市: %s (%s, %s)", loc.Name, loc.Adm2, loc.Adm1)
	return loc.ID, loc.Name, nil
}

func (t *WeatherTool) getNow(ctx context.Context, locationID string) (string, error) {
	u := fmt.Sprintf("https://%s/v7/weather/now?location=%s",
		t.apiHost, locationID)

	body, err := t.doGet(ctx, u)
	if err != nil {
		return "", fmt.Errorf("实时天气查询失败: %w", err)
	}

	var resp qweatherNowResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析天气数据失败: %w", err)
	}

	if resp.Code != "200" {
		return "", fmt.Errorf("天气API错误 code=%s", resp.Code)
	}

	n := resp.Now
	return fmt.Sprintf("实时: %s, 温度%s°C, 体感%s°C, %s%s级, 湿度%s%%",
		n.Text, n.Temp, n.FeelsLike, n.WindDir, n.WindScale, n.Humidity), nil
}

func (t *WeatherTool) getForecast(ctx context.Context, locationID string) (string, error) {
	u := fmt.Sprintf("https://%s/v7/weather/3d?location=%s",
		t.apiHost, locationID)

	body, err := t.doGet(ctx, u)
	if err != nil {
		return "", fmt.Errorf("天气预报查询失败: %w", err)
	}

	var resp qweatherForecastResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析预报数据失败: %w", err)
	}

	if resp.Code != "200" {
		return "", fmt.Errorf("预报API错误 code=%s", resp.Code)
	}

	var lines []string
	for _, d := range resp.Daily {
		lines = append(lines, fmt.Sprintf("%s: %s转%s, %s~%s°C, %s%s级",
			d.FxDate, d.TextDay, d.TextNight, d.TempMin, d.TempMax, d.WindDirDay, d.WindScaleDay))
	}
	return "预报:\n" + joinLines(lines), nil
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}

// geoHost 返回 Geo API 的 host。
// 和风天气的免费订阅使用相同 host。
func (t *WeatherTool) geoHost() string {
	return t.apiHost
}

func (t *WeatherTool) doGet(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	// JWT 认证优先，否则回退到 API Key
	if t.useJWT {
		token, err := t.getToken()
		if err != nil {
			return nil, fmt.Errorf("生成 JWT token 失败: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		req.Header.Set("X-QW-Api-Key", t.apiKey)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
