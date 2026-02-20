package voiceprint

import (
	"fmt"
	"math"
	"sync"

	"github.com/iabetor/pibuddy/internal/logger"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// Extractor 封装 sherpa-onnx SpeakerEmbeddingExtractor，用于从音频提取声纹向量。
type Extractor struct {
	impl *sherpa.SpeakerEmbeddingExtractor
	mu   sync.Mutex
}

// NewExtractor 创建声纹提取器。
// modelPath: ONNX 模型文件路径
// numThreads: 推理线程数
func NewExtractor(modelPath string, numThreads int) (*Extractor, error) {
	config := &sherpa.SpeakerEmbeddingExtractorConfig{
		Model:      modelPath,
		NumThreads: numThreads,
		Debug:      0,
		Provider:   "cpu",
	}

	impl := sherpa.NewSpeakerEmbeddingExtractor(config)
	if impl == nil {
		return nil, fmt.Errorf("创建声纹提取器失败，模型路径: %s", modelPath)
	}

	logger.Infof("[voiceprint] 声纹提取器已初始化 (model=%s, dim=%d)", modelPath, impl.Dim())

	return &Extractor{impl: impl}, nil
}

// Extract 从音频样本中提取 embedding 向量。
// samples: 16kHz 单声道 float32 音频数据
func (e *Extractor) Extract(samples []float32) ([]float32, error) {
	// 预处理：过滤静音段，只保留有效语音
	processed := preprocessAudio(samples)
	if len(processed) < 8000 { // 至少 0.5 秒有效音频
		return nil, fmt.Errorf("有效音频数据不足（%d 采样点），可能环境太安静或录制质量不佳", len(processed))
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	stream := e.impl.CreateStream()
	if stream == nil {
		return nil, fmt.Errorf("创建声纹提取流失败")
	}
	defer sherpa.DeleteOnlineStream(stream)

	stream.AcceptWaveform(16000, processed)
	stream.InputFinished()

	if !e.impl.IsReady(stream) {
		return nil, fmt.Errorf("音频数据不足，无法提取声纹")
	}

	embedding := e.impl.Compute(stream)
	return embedding, nil
}

// preprocessAudio 对音频进行预处理：
// 1. 计算能量阈值，过滤静音帧
// 2. 对有效帧做能量归一化
func preprocessAudio(samples []float32) []float32 {
	if len(samples) == 0 {
		return samples
	}

	const frameSize = 400 // 25ms @ 16kHz
	const hopSize = 160   // 10ms @ 16kHz

	// 计算每帧能量
	type frameInfo struct {
		start  int
		end    int
		energy float64
	}
	var frames []frameInfo
	var maxEnergy float64

	for start := 0; start+frameSize <= len(samples); start += hopSize {
		end := start + frameSize
		var energy float64
		for _, s := range samples[start:end] {
			energy += float64(s) * float64(s)
		}
		energy /= float64(frameSize)
		if energy > maxEnergy {
			maxEnergy = energy
		}
		frames = append(frames, frameInfo{start, end, energy})
	}

	if maxEnergy == 0 || len(frames) == 0 {
		return samples
	}

	// 静音阈值：最大能量的 2%（比较宽松，保留更多语音）
	silenceThreshold := maxEnergy * 0.02

	// 收集有效帧
	var result []float32
	for _, f := range frames {
		if f.energy >= silenceThreshold {
			result = append(result, samples[f.start:f.end]...)
		}
	}

	if len(result) == 0 {
		return samples // 全静音就返回原始数据
	}

	// 能量归一化：将 RMS 归一化到目标水平
	var rms float64
	for _, s := range result {
		rms += float64(s) * float64(s)
	}
	rms = math.Sqrt(rms / float64(len(result)))

	if rms > 1e-6 {
		targetRMS := 0.1 // 目标 RMS
		scale := float32(targetRMS / rms)
		for i := range result {
			result[i] *= scale
			// 限幅
			if result[i] > 1.0 {
				result[i] = 1.0
			} else if result[i] < -1.0 {
				result[i] = -1.0
			}
		}
	}

	logger.Debugf("[voiceprint] 音频预处理: %d → %d 采样点 (保留 %.0f%%)",
		len(samples), len(result), float64(len(result))/float64(len(samples))*100)

	return result
}

// Dim 返回 embedding 向量维度。
func (e *Extractor) Dim() int {
	return e.impl.Dim()
}

// Close 释放底层资源。
func (e *Extractor) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.impl != nil {
		sherpa.DeleteSpeakerEmbeddingExtractor(e.impl)
		e.impl = nil
	}

	logger.Info("[voiceprint] 声纹提取器已关闭")
}
