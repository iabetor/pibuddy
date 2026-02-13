package vad

import (
	"fmt"
	"log"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// Detector 封装 sherpa-onnx Silero VAD，用于语音活动检测（端点检测）。
type Detector struct {
	vad    *sherpa.VoiceActivityDetector
	config sherpa.VadModelConfig
}

// NewDetector 创建语音活动检测器。
// modelPath: Silero VAD ONNX 模型文件路径
// threshold: 检测灵敏度（典型值 0.5）
// minSilenceMs: 最小静音时长（毫秒），超过此时长视为说话结束
func NewDetector(modelPath string, threshold float32, minSilenceMs int) (*Detector, error) {
	config := sherpa.VadModelConfig{
		SileroVad: sherpa.SileroVadModelConfig{
			Model:              modelPath,
			Threshold:          threshold,
			MinSilenceDuration: float32(minSilenceMs) / 1000.0,
			MinSpeechDuration:  0.1,
			MaxSpeechDuration:  30.0,
			WindowSize:         512,
		},
		SampleRate: 16000,
		NumThreads: 1,
		Provider:   "cpu",
	}

	// NewVoiceActivityDetector 的第二个参数是 float32（缓冲区秒数）
	vad := sherpa.NewVoiceActivityDetector(&config, float32(30))
	if vad == nil {
		return nil, fmt.Errorf("创建语音活动检测器失败，模型: %s", modelPath)
	}

	log.Printf("[vad] 语音活动检测器已创建: model=%s threshold=%.2f minSilenceMs=%d",
		modelPath, threshold, minSilenceMs)

	return &Detector{
		vad:    vad,
		config: config,
	}, nil
}

// Feed 将音频样本送入 VAD 进行处理。
// 样本应为 16kHz float32 格式。
func (d *Detector) Feed(samples []float32) {
	d.vad.AcceptWaveform(samples)
}

// IsSpeech 返回当前是否检测到语音。
func (d *Detector) IsSpeech() bool {
	return d.vad.IsSpeech()
}

// Flush 刷新 VAD 以完成任何挂起的语音段。
func (d *Detector) Flush() {
	d.vad.Flush()
}

// GetSegment 获取下一个可用的语音片段。
// 如果有语音片段，返回 (samples, true)；否则返回 (nil, false)。
func (d *Detector) GetSegment() ([]float32, bool) {
	if d.vad.IsEmpty() {
		return nil, false
	}

	segment := d.vad.Front()
	d.vad.Pop()

	return segment.Samples, true
}

// Reset 重置 VAD 内部状态，为处理新的音频流做准备。
func (d *Detector) Reset() {
	d.vad.Clear()
}

// Close 释放底层 sherpa-onnx VAD 资源。
func (d *Detector) Close() {
	if d.vad != nil {
		sherpa.DeleteVoiceActivityDetector(d.vad)
		d.vad = nil
		log.Println("[vad] 语音活动检测器已关闭")
	}
}
