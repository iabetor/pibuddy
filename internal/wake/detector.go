package wake

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// Detector 封装 sherpa-onnx 关键词检测（KWS），用于唤醒词检测。
type Detector struct {
	spotter *sherpa.KeywordSpotter
	stream  *sherpa.OnlineStream
	mu      sync.Mutex
}

// NewDetector 创建唤醒词检测器。
// modelPath: 包含 encoder.onnx、decoder.onnx、joiner.onnx、tokens.txt 的目录
// keywords: 唤醒词配置（如关键词文件内容或关键词字符串）
// threshold: 检测灵敏度（0-1，越低越灵敏）
func NewDetector(modelPath, keywords string, threshold float32) (*Detector, error) {
	config := sherpa.KeywordSpotterConfig{}

	// 特征提取配置
	config.FeatConfig.SampleRate = 16000
	config.FeatConfig.FeatureDim = 80

	// Transducer 模型文件路径
	config.ModelConfig.Transducer.Encoder = filepath.Join(modelPath, "encoder.onnx")
	config.ModelConfig.Transducer.Decoder = filepath.Join(modelPath, "decoder.onnx")
	config.ModelConfig.Transducer.Joiner = filepath.Join(modelPath, "joiner.onnx")

	// 词表和运行时配置
	config.ModelConfig.Tokens = filepath.Join(modelPath, "tokens.txt")
	config.ModelConfig.NumThreads = 2
	config.ModelConfig.Provider = "cpu"

	// 关键词配置
	config.KeywordsBuf = keywords
	config.KeywordsThreshold = threshold

	spotter := sherpa.NewKeywordSpotter(&config)
	if spotter == nil {
		return nil, fmt.Errorf("创建关键词检测器失败，模型路径: %s", modelPath)
	}

	stream := sherpa.NewKeywordStream(spotter)
	if stream == nil {
		sherpa.DeleteKeywordSpotter(spotter)
		return nil, fmt.Errorf("创建关键词检测流失败")
	}

	log.Printf("[wake] 唤醒词检测器已初始化 (model=%s, threshold=%.2f)", modelPath, threshold)

	return &Detector{
		spotter: spotter,
		stream:  stream,
	}, nil
}

// Detect 将音频样本送入关键词检测器，检测到唤醒词时返回 true。
// 检测到后会自动重置流，准备下一次检测。
func (d *Detector) Detect(samples []float32) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stream.AcceptWaveform(16000, samples)

	for d.spotter.IsReady(d.stream) {
		d.spotter.Decode(d.stream)
		result := d.spotter.GetResult(d.stream)
		if result.Keyword != "" {
			log.Printf("[wake] 检测到唤醒词: %s", result.Keyword)
			d.spotter.Reset(d.stream)
			return true
		}
	}

	return false
}

// Close 释放底层 sherpa-onnx 资源。
func (d *Detector) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stream != nil {
		sherpa.DeleteOnlineStream(d.stream)
		d.stream = nil
	}
	if d.spotter != nil {
		sherpa.DeleteKeywordSpotter(d.spotter)
		d.spotter = nil
	}

	log.Println("[wake] 唤醒词检测器已关闭")
}
