package asr

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	asr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/asr/v20190614"
)

// TencentFlashEngine 腾讯云一句话识别引擎。
// 适用于 ≤60 秒的短语音识别，每月 5000 次免费额度。
// 文档：https://cloud.tencent.com/document/product/1093/35646
//
// 重要：一句话识别是批处理模式，GetResult() 只在端点触发后才调用 API。
// pipeline 中每帧都会调用 GetResult()（用于获取实时中间结果），
// 但本引擎在非端点触发场景下返回空字符串，不发起 HTTP 请求。
type TencentFlashEngine struct {
	client      *asr.Client

	// 音频缓冲
	mu          sync.Mutex
	buffer      *bytes.Buffer
	sampleRate  int

	// 批处理控制：只在端点触发后才发起 API 调用
	pendingRecognize bool // 是否有待识别的请求（由 FallbackEngine 在 IsEndpoint 后设置）

	// 异步识别结果
	asyncResult  string // 异步识别返回的结果
	asyncRunning bool   // 是否正在异步识别中
	asyncErr     error  // 异步识别错误

	// 状态
	status      EngineStatus
	lastError   error
	lastErrorAt time.Time
}

// TencentFlashConfig 腾讯云一句话识别配置
type TencentFlashConfig struct {
	SecretID  string
	SecretKey string
	Region    string // 默认 ap-guangzhou
}

// NewTencentFlashEngine 创建腾讯云一句话识别引擎。
func NewTencentFlashEngine(cfg TencentFlashConfig) (*TencentFlashEngine, error) {
	if cfg.SecretID == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("腾讯云 SecretID 和 SecretKey 不能为空")
	}

	region := cfg.Region
	if region == "" {
		region = "ap-guangzhou"
	}

	// 使用腾讯云 SDK 创建客户端
	credential := common.NewCredential(cfg.SecretID, cfg.SecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "asr.tencentcloudapi.com"

	client, err := asr.NewClient(credential, region, cpf)
	if err != nil {
		return nil, fmt.Errorf("创建腾讯云 ASR 客户端失败: %w", err)
	}

	e := &TencentFlashEngine{
		client:     client,
		buffer:     bytes.NewBuffer(nil),
		sampleRate: 16000,
		status:     StatusAvailable,
	}

	logger.Infof("[asr] 腾讯云一句话识别引擎已初始化 (region=%s)", region)
	return e, nil
}

// Feed 实现 Engine 接口。
// 将音频样本缓存到缓冲区，等待 IsEndpoint 后统一识别。
func (e *TencentFlashEngine) Feed(samples []float32) {
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
func (e *TencentFlashEngine) GetResult() string {
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
		logger.Errorf("[asr] 腾讯云一句话识别失败: %v", e.asyncErr)
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

		if e.buffer.Len() == 0 {
			return ""
		}

		audioData := make([]byte, e.buffer.Len())
		copy(audioData, e.buffer.Bytes())

		// 裁剪尾部静音，减少发送给 API 的音频时长
		audioData = trimTrailingSilencePCM(audioData, e.sampleRate)

		// 启动异步识别
		e.asyncRunning = true
		go func() {
			result, err := e.recognize(audioData)

			e.mu.Lock()
			defer e.mu.Unlock()
			e.asyncRunning = false

			if err != nil {
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

// TriggerRecognize 设置待识别标记，在下次 GetResult 时发起 API 调用。
// 由 FallbackEngine 在端点检测触发后调用。
func (e *TencentFlashEngine) TriggerRecognize() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pendingRecognize = true
}

// IsEndpoint 实现 Engine 接口。
// 一句话识别引擎本身不做端点检测，由 VAD 或调用者决定。
// 这里始终返回 false，由 pipeline 的 VAD 决定何时识别。
func (e *TencentFlashEngine) IsEndpoint() bool {
	return false
}

// Reset 实现 Engine 接口。
func (e *TencentFlashEngine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.buffer.Reset()
	e.pendingRecognize = false
	e.asyncResult = ""
	e.asyncErr = nil
}

// Close 实现 Engine 接口。
func (e *TencentFlashEngine) Close() {
	logger.Info("[asr] 腾讯云一句话识别引擎已关闭")
}

// Name 实现 Engine 接口。
func (e *TencentFlashEngine) Name() string {
	return string(EngineTencentFlash)
}

// Status 实现 StatusEngine 接口。
func (e *TencentFlashEngine) Status() EngineStatus {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 如果最近有错误，检查是否已过冷却期
	if e.status == StatusDegraded && !e.lastErrorAt.IsZero() {
		if time.Since(e.lastErrorAt) > 5*time.Minute {
			e.status = StatusAvailable
		}
	}

	return e.status
}

// recognize 调用腾讯云一句话识别 API。
func (e *TencentFlashEngine) recognize(audioData []byte) (string, error) {
	// 计算音频时长（秒）
	audioDuration := float64(len(audioData)/2) / float64(e.sampleRate)

	// 使用 SDK 调用一句话识别
	req := asr.NewSentenceRecognitionRequest()
	req.EngSerViceType = common.StringPtr("16k_zh") // 中文通用
	sourceType := uint64(1) // 语音数据来源为语音数据（base64 编码）
	req.SourceType = &sourceType
	req.VoiceFormat = common.StringPtr("pcm") // PCM 格式
	req.Data = common.StringPtr(base64.StdEncoding.EncodeToString(audioData))
	req.DataLen = common.Int64Ptr(int64(len(audioData)))

	resp, err := e.client.SentenceRecognition(req)
	if err != nil {
		return "", fmt.Errorf("调用腾讯云一句话识别 API 失败: %w", err)
	}

	if resp.Response == nil || resp.Response.Result == nil {
		return "", fmt.Errorf("腾讯云返回空结果")
	}

	result := *resp.Response.Result
	logger.Debugf("[asr] 腾讯云一句话识别成功: %s (时长: %.2fs)", result, audioDuration)

	return strings.TrimSpace(result), nil
}
