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
// modelPath: 包含 encoder/decoder/joiner onnx 和 tokens.txt 的目录
// keywordsFile: 关键词文件路径（拼音 token 格式）
// threshold: 检测灵敏度（0-1，越低越灵敏）
func NewDetector(modelPath, keywordsFile string, threshold float32) (*Detector, error) {
	config := sherpa.KeywordSpotterConfig{}

	// 特征提取配置
	config.FeatConfig.SampleRate = 16000
	config.FeatConfig.FeatureDim = 80

	// Transducer 模型文件路径（使用 int8 量化版本，更适合树莓派）
	config.ModelConfig.Transducer.Encoder = filepath.Join(modelPath, "encoder-epoch-12-avg-2-chunk-16-left-64.int8.onnx")
	config.ModelConfig.Transducer.Decoder = filepath.Join(modelPath, "decoder-epoch-12-avg-2-chunk-16-left-64.int8.onnx")
	config.ModelConfig.Transducer.Joiner = filepath.Join(modelPath, "joiner-epoch-12-avg-2-chunk-16-left-64.int8.onnx")

	// 词表和运行时配置
	config.ModelConfig.Tokens = filepath.Join(modelPath, "tokens.txt")
	config.ModelConfig.NumThreads = 2
	config.ModelConfig.Provider = "cpu"

	// 关键词配置
	config.KeywordsFile = keywordsFile
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

// Reset 清空检测器的内部缓冲区，用于防止重复检测。
func (d *Detector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.spotter != nil && d.stream != nil {
		d.spotter.Reset(d.stream)
	}
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
