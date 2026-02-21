package asr

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/iabetor/pibuddy/internal/logger"
)

// TencentRTEngine 腾讯云实时语音识别引擎。
// 使用 WebSocket 协议实现边说边转，每月 5 小时免费额度。
// 文档：https://cloud.tencent.com/document/product/1093/48982
//
// 重要：虽然实时语音识别支持流式，但当前实现是端点触发后批量发送，
// 与 FallbackEngine 的 BatchEngine 接口配合使用。
type TencentRTEngine struct {
	secretID    string
	secretKey   string
	region      string
	appID       string // 腾讯云 APPID

	// 音频缓冲
	mu          sync.Mutex
	buffer      *bytes.Buffer
	sampleRate  int

	// 批处理控制：只在端点触发后才发起 WebSocket 调用
	pendingRecognize bool

	// 异步识别结果
	asyncResult   string // 异步识别返回的结果
	asyncRunning  bool   // 是否正在异步识别中
	asyncErr      error  // 异步识别错误

	// 取消控制
	cancel context.CancelFunc // 用于取消正在进行的识别

	// 状态
	status      EngineStatus
	lastError   error
	lastErrorAt time.Time

	// WebSocket 连接
	conn        *websocket.Conn
	connMu      sync.Mutex
	currentText strings.Builder
	engineModel string // 引擎模型类型，如 16k_zh
}

// TencentRTConfig 腾讯云实时语音识别配置
type TencentRTConfig struct {
	SecretID  string
	SecretKey string
	Region    string // 默认 ap-guangzhou
	AppID     string // 腾讯云 APPID
}

// NewTencentRTEngine 创建腾讯云实时语音识别引擎。
func NewTencentRTEngine(cfg TencentRTConfig) (*TencentRTEngine, error) {
	if cfg.SecretID == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("腾讯云 SecretID 和 SecretKey 不能为空")
	}
	if cfg.AppID == "" {
		return nil, fmt.Errorf("腾讯云 AppID 不能为空")
	}

	region := cfg.Region
	if region == "" {
		region = "ap-guangzhou"
	}

	e := &TencentRTEngine{
		secretID:    cfg.SecretID,
		secretKey:   cfg.SecretKey,
		region:      region,
		appID:       cfg.AppID,
		buffer:      bytes.NewBuffer(nil),
		sampleRate:  16000,
		status:      StatusAvailable,
		engineModel: "16k_zh",
	}

	logger.Infof("[asr] 腾讯云实时语音识别引擎已初始化 (region=%s)", region)
	return e, nil
}

// Feed 实现 Engine 接口。
// 将音频样本缓存到缓冲区。
func (e *TencentRTEngine) Feed(samples []float32) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 转换 float32 到 int16（PCM）
	for _, sample := range samples {
		val := int16(sample * 32767)
		e.buffer.WriteByte(byte(val))
		e.buffer.WriteByte(byte(val >> 8))
	}
}

// GetResult 实现 Engine 接口。
// 非阻塞：pendingRecognize 触发后启动异步识别 goroutine，
// 后续每帧轮询检查异步结果是否就绪。
func (e *TencentRTEngine) GetResult() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. 检查是否有异步识别结果
	if e.asyncResult != "" {
		result := e.asyncResult
		e.asyncResult = ""
		return result
	}

	// 2. 如果异步识别出错，记录状态
	if e.asyncErr != nil {
		logger.Errorf("[asr] 腾讯云实时语音识别失败: %v", e.asyncErr)
		e.lastError = e.asyncErr
		e.lastErrorAt = time.Now()
		if IsQuotaExhaustedError(e.asyncErr) || IsNetworkError(e.asyncErr) {
			e.status = StatusDegraded
		}
		e.asyncErr = nil
		return ""
	}

	// 3. 如果需要触发识别且当前没有异步任务在运行
	if e.pendingRecognize && !e.asyncRunning {
		e.pendingRecognize = false

		audioData := make([]byte, e.buffer.Len())
		copy(audioData, e.buffer.Bytes())

		if len(audioData) == 0 {
			return ""
		}

		// 裁剪尾部静音，减少发送给 API 的音频时长
		audioData = trimTrailingSilencePCM(audioData, e.sampleRate)

		// 创建可取消的 context
		ctx, cancel := context.WithCancel(context.Background())
		e.cancel = cancel

		// 启动异步识别
		e.asyncRunning = true
		go func() {
			result, err := e.recognize(ctx, audioData)

			e.mu.Lock()
			defer e.mu.Unlock()
			e.asyncRunning = false
			e.cancel = nil

			if err != nil && err != context.Canceled {
				e.asyncErr = err
				return
			}

			// 成功时清空 buffer 并保存结果
			e.buffer.Reset()
			e.asyncResult = result
		}()
	}

	return ""
}

// TriggerRecognize 设置待识别标记，在下次 GetResult 时启动异步识别。
// 由 FallbackEngine 在端点检测触发后调用。
func (e *TencentRTEngine) TriggerRecognize() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pendingRecognize = true
}

// IsEndpoint 实现 Engine 接口。
// 实时语音识别引擎本身不做端点检测。
func (e *TencentRTEngine) IsEndpoint() bool {
	return false
}

// Reset 实现 Engine 接口。
func (e *TencentRTEngine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.buffer.Reset()
	e.currentText.Reset()
	e.pendingRecognize = false
	e.asyncResult = ""
	e.asyncErr = nil
	// 注意：不清理 asyncRunning，让正在运行的 goroutine 自然结束
}

// Cancel 取消正在进行的识别。
func (e *TencentRTEngine) Cancel() {
	e.mu.Lock()
	defer e.mu.Unlock()
	logger.Debugf("[asr] 腾讯云实时语音识别 Cancel() 被调用, cancel=%v, asyncRunning=%v", e.cancel != nil, e.asyncRunning)
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
		logger.Debug("[asr] 腾讯云实时语音识别已取消")
	}
}

// Close 实现 Engine 接口。
func (e *TencentRTEngine) Close() {
	e.connMu.Lock()
	if e.conn != nil {
		e.conn.Close()
		e.conn = nil
	}
	e.connMu.Unlock()
	logger.Info("[asr] 腾讯云实时语音识别引擎已关闭")
}

// Name 实现 Engine 接口。
func (e *TencentRTEngine) Name() string {
	return string(EngineTencentRT)
}

// Status 实现 StatusEngine 接口。
func (e *TencentRTEngine) Status() EngineStatus {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.status == StatusDegraded && !e.lastErrorAt.IsZero() {
		if time.Since(e.lastErrorAt) > 5*time.Minute {
			e.status = StatusAvailable
		}
	}

	return e.status
}

// recognize 使用 WebSocket 进行实时语音识别。
// 注意：此方法在 goroutine 中调用，不阻塞主循环。
func (e *TencentRTEngine) recognize(ctx context.Context, audioData []byte) (string, error) {
	// 构建 WebSocket URL
	wsURL, err := e.buildWebSocketURL()
	if err != nil {
		return "", err
	}

	logger.Debugf("[asr] 腾讯云实时语音识别: 发送 %d 字节音频", len(audioData))

	// 建立 WebSocket 连接
	e.connMu.Lock()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		e.connMu.Unlock()
		return "", fmt.Errorf("WebSocket 连接失败: %w", err)
	}
	e.conn = conn
	e.connMu.Unlock()

	defer func() {
		e.connMu.Lock()
		if e.conn != nil {
			e.conn.Close()
			e.conn = nil
		}
		e.connMu.Unlock()
	}()

	// 启动结果读取 goroutine
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)
	var resultText strings.Builder

	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}

			var resp RTASRResponse
			if err := json.Unmarshal(message, &resp); err != nil {
				continue
			}

			// 解析结果
			if resp.Code != 0 {
				errChan <- fmt.Errorf("ASR 错误 (code=%d): %s", resp.Code, resp.Message)
				return
			}

			// 提取文本（slice_type=2 表示一句话最终结果）
			if resp.Result != nil && resp.Result.SliceType == 2 {
				resultText.WriteString(resp.Result.VoiceTextStr)
			}

			// 最终结果
			if resp.Final == 1 {
				resultChan <- resultText.String()
				return
			}
		}
	}()

	// 发送音频数据（不需要模拟实时率，批量快速发送）
	chunkSize := 6400 // 200ms @ 16kHz 16bit = 6400 bytes
	for i := 0; i < len(audioData); i += chunkSize {
		// 检查是否已取消
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		end := i + chunkSize
		if end > len(audioData) {
			end = len(audioData)
		}

		if err := conn.WriteMessage(websocket.BinaryMessage, audioData[i:end]); err != nil {
			return "", fmt.Errorf("发送音频失败: %w", err)
		}
	}

	// 检查是否已取消
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// 发送结束帧
	endSignal := map[string]interface{}{"type": "end"}
	endData, _ := json.Marshal(endSignal)
	if err := conn.WriteMessage(websocket.TextMessage, endData); err != nil {
		return "", fmt.Errorf("发送结束信号失败: %w", err)
	}

	// 等待结果
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-resultChan:
		result = strings.TrimSpace(result)
		if result != "" {
			logger.Infof("[asr] 腾讯云实时语音识别结果: %s", result)
		}
		return result, nil
	case err := <-errChan:
		return "", err
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("识别超时")
	}
}

// buildWebSocketURL 构建 WebSocket 连接 URL。
// 参考文档：https://cloud.tencent.com/document/product/1093/48982
func (e *TencentRTEngine) buildWebSocketURL() (string, error) {
	host := "asr.cloud.tencent.com"
	path := fmt.Sprintf("/asr/v2/%s", e.appID)

	now := time.Now().Unix()

	// 必填参数（不含 signature）
	params := map[string]string{
		"secretid":          e.secretID,
		"timestamp":         fmt.Sprintf("%d", now),
		"expired":           fmt.Sprintf("%d", now+86400), // 24 小时有效
		"nonce":             fmt.Sprintf("%d", rand.Intn(99999-1000)+1000),
		"engine_model_type": e.engineModel,
		"voice_id":          uuid.New().String(),
		"voice_format":      "1", // PCM
		"needvad":           "1", // 启用 VAD
	}

	// 1. 按字典序排列参数，构建签名原文
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sortedParams []string
	for _, k := range keys {
		sortedParams = append(sortedParams, fmt.Sprintf("%s=%s", k, params[k]))
	}
	queryStr := strings.Join(sortedParams, "&")

	// 签名原文 = host + path + ? + sortedQuery（不含 wss://）
	signStr := fmt.Sprintf("%s%s?%s", host, path, queryStr)

	// 2. HMAC-SHA1 签名
	signature := e.hmacSHA1(signStr)

	// 3. URL 编码签名
	encodedSignature := url.QueryEscape(signature)

	// 4. 构建完整 URL
	wsURL := fmt.Sprintf("wss://%s%s?%s&signature=%s", host, path, queryStr, encodedSignature)
	return wsURL, nil
}

// hmacSHA1 计算 HMAC-SHA1 签名并返回 Base64 编码。
func (e *TencentRTEngine) hmacSHA1(data string) string {
	h := hmac.New(sha1.New, []byte(e.secretKey))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// RTASRResponse 实时语音识别响应结构
type RTASRResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	VoiceID   string `json:"voice_id"`
	MessageID string `json:"message_id"`
	Result    *struct {
		VoiceTextStr string `json:"voice_text_str"`
		SliceType    int    `json:"slice_type"` // 0=一句话开始，1=中间结果，2=一句话结束
	} `json:"result"`
	Final int `json:"final"` // 1=最终结果
}

// float32ToBytes 将 []float32 转换为 PCM int16 字节
func float32ToPCM(samples []float32) []byte {
	buf := make([]byte, len(samples)*2)
	for i, sample := range samples {
		val := int16(sample * 32767)
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(val))
	}
	return buf
}
