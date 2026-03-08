package tts

import (
	"context"
	"fmt"

	"github.com/iabetor/pibuddy/internal/logger"
	"github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// SherpaEngine 使用 sherpa-onnx 实现离线语音合成。
type SherpaEngine struct {
	tts   *sherpa_onnx.OfflineTts
	speed float32
}

// SherpaConfig Sherpa TTS 配置。
type SherpaConfig struct {
	ModelPath   string  // 模型文件路径 (.onnx)
	TokensPath  string  // tokens 文件路径
	LexiconPath string  // lexicon 文件路径（可选）
	DataDir     string  // espeak-ng-data 目录（可选）
	NoiseScale  float32 // 默认 0.667
	LengthScale float32 // 默认 1.0，越小越快
	NoiseScaleW float32 // 默认 0.8
	Speed       float32 // 语速，1.0 为正常
}

// NewSherpaEngine 创建 Sherpa-onnx TTS 引擎。
func NewSherpaEngine(cfg SherpaConfig) (*SherpaEngine, error) {
	// 设置默认值
	noiseScale := cfg.NoiseScale
	if noiseScale == 0 {
		noiseScale = 0.667
	}
	lengthScale := cfg.LengthScale
	if lengthScale == 0 {
		lengthScale = 1.0
	}
	noiseScaleW := cfg.NoiseScaleW
	if noiseScaleW == 0 {
		noiseScaleW = 0.8
	}
	speed := cfg.Speed
	if speed == 0 {
		speed = 1.0
	}

	// 创建 TTS 配置
	ttsConfig := &sherpa_onnx.OfflineTtsConfig{
		Model: sherpa_onnx.OfflineTtsModelConfig{
			Vits: sherpa_onnx.OfflineTtsVitsModelConfig{
				Model:       cfg.ModelPath,
				Tokens:      cfg.TokensPath,
				Lexicon:     cfg.LexiconPath,
				DataDir:     cfg.DataDir,
				NoiseScale:  noiseScale,
				LengthScale: lengthScale,
				NoiseScaleW: noiseScaleW,
			},
			NumThreads: 2,
			Debug:      0,
			Provider:   "cpu",
		},
		MaxNumSentences: 1,
	}

	tts := sherpa_onnx.NewOfflineTts(ttsConfig)
	if tts == nil {
		return nil, fmt.Errorf("[tts] sherpa-onnx 创建 TTS 失败")
	}

	logger.Infof("[tts] sherpa-onnx TTS 引擎已初始化，模型=%s", cfg.ModelPath)

	return &SherpaEngine{tts: tts, speed: speed}, nil
}

// Synthesize 将文本转换为音频。
func (e *SherpaEngine) Synthesize(ctx context.Context, text string) ([]float32, int, error) {
	logger.Debugf("[tts] sherpa-onnx: 正在合成 %d 个字符", len([]rune(text)))

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}

	// 生成音频（sid=0, speed=1.0）
	audio := e.tts.Generate(text, 0, e.speed)
	if audio == nil || len(audio.Samples) == 0 {
		return nil, 0, fmt.Errorf("[tts] sherpa-onnx: 未生成音频数据")
	}

	logger.Debugf("[tts] sherpa-onnx: 生成 %d 个样本，采样率 %d Hz", len(audio.Samples), audio.SampleRate)

	return audio.Samples, audio.SampleRate, nil
}

// Close 释放资源。
func (e *SherpaEngine) Close() {
	if e.tts != nil {
		sherpa_onnx.DeleteOfflineTts(e.tts)
	}
}
