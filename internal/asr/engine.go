package asr

import (
	"encoding/binary"
	"math"

	"github.com/iabetor/pibuddy/internal/logger"
)

// Engine 定义 ASR 引擎的统一接口。
// 支持多种后端实现（sherpa-onnx、腾讯云等）和多层兜底。
type Engine interface {
	// Feed 将音频样本送入识别引擎。
	// 样本应为 16kHz float32 格式。
	Feed(samples []float32)

	// GetResult 返回当前识别文本。
	GetResult() string

	// IsEndpoint 返回是否检测到端点（说话者已结束一句话）。
	IsEndpoint() bool

	// Reset 重置识别状态，为处理新语句做准备。
	Reset()

	// Close 释放资源。
	Close()

	// Name 返回引擎名称，用于日志和调试。
	Name() string
}

// EngineStatus 引擎状态
type EngineStatus int

const (
	StatusAvailable EngineStatus = iota // 可用
	StatusDegraded                      // 降级（额度不足等）
	StatusUnavailable                   // 不可用
)

// StatusEngine 是带状态查询的引擎接口（可选实现）
type StatusEngine interface {
	Engine
	Status() EngineStatus
}

// BatchEngine 是批处理模式的引擎接口（可选实现）。
// 批处理引擎（如腾讯云一句话识别）不支持实时中间结果，
// 需要在端点触发后显式调用 TriggerRecognize() 才会发起 API 调用。
type BatchEngine interface {
	Engine
	TriggerRecognize()
}

// EngineType 引擎类型
type EngineType string

const (
	EngineSherpa       EngineType = "sherpa"       // 离线引擎
	EngineTencentFlash EngineType = "tencent-flash" // 腾讯云一句话识别
	EngineTencentRT    EngineType = "tencent-rt"    // 腾讯云实时语音识别
)

// IsOnline 返回是否为在线引擎
func (t EngineType) IsOnline() bool {
	return t == EngineTencentFlash || t == EngineTencentRT
}

// logEngineSwitch 记录引擎切换
func logEngineSwitch(from, to EngineType, reason string) {
	logger.Warnf("[asr] 引擎切换: %s -> %s (%s)", from, to, reason)
}

// logEngineAvailable 记录引擎可用
func logEngineAvailable(engine EngineType) {
	logger.Infof("[asr] 引擎 %s 已就绪", engine)
}

// trimTrailingSilencePCM 裁剪 PCM 音频数据（16bit LE）的尾部静音。
// 从尾部向前扫描，找到最后一个超过阈值的采样点，保留其后 200ms 的数据。
// 这样可以避免将端点检测等待的静音部分发送给在线 ASR API，节省 API 时间额度。
func trimTrailingSilencePCM(audioData []byte, sampleRate int) []byte {
	if len(audioData) < 2 {
		return audioData
	}

	// 最少保留 500ms 的音频
	minSamples := sampleRate / 2
	minBytes := minSamples * 2
	if len(audioData) <= minBytes {
		return audioData
	}

	// 静音阈值：int16 范围 [-32768, 32767]，阈值取 300（约 -40dB）
	const silenceThreshold int16 = 300
	// 尾部保留 200ms
	trailingSamples := sampleRate / 5

	numSamples := len(audioData) / 2

	// 从尾部向前扫描，找到最后一个非静音采样
	lastNonSilent := -1
	for i := numSamples - 1; i >= 0; i-- {
		sample := int16(binary.LittleEndian.Uint16(audioData[i*2 : i*2+2]))
		if int16(math.Abs(float64(sample))) > silenceThreshold {
			lastNonSilent = i
			break
		}
	}

	if lastNonSilent < 0 {
		// 全是静音，返回原数据（让 API 自己处理）
		return audioData
	}

	// 保留非静音点后 200ms
	endSample := lastNonSilent + trailingSamples
	if endSample >= numSamples {
		return audioData // 没有多少尾部静音，不裁剪
	}

	trimmedBytes := (endSample + 1) * 2
	trimmedDuration := float64(endSample+1) / float64(sampleRate)
	originalDuration := float64(numSamples) / float64(sampleRate)
	logger.Debugf("[asr] 裁剪尾部静音: %.1fs → %.1fs (节省 %.1fs)",
		originalDuration, trimmedDuration, originalDuration-trimmedDuration)

	return audioData[:trimmedBytes]
}
