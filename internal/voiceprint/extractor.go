package voiceprint

import (
	"fmt"
	"log"
	"sync"

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

	log.Printf("[voiceprint] 声纹提取器已初始化 (model=%s, dim=%d)", modelPath, impl.Dim())

	return &Extractor{impl: impl}, nil
}

// Extract 从音频样本中提取 embedding 向量。
// samples: 16kHz 单声道 float32 音频数据
func (e *Extractor) Extract(samples []float32) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	stream := e.impl.CreateStream()
	if stream == nil {
		return nil, fmt.Errorf("创建声纹提取流失败")
	}
	defer sherpa.DeleteOnlineStream(stream)

	stream.AcceptWaveform(16000, samples)
	stream.InputFinished()

	if !e.impl.IsReady(stream) {
		return nil, fmt.Errorf("音频数据不足，无法提取声纹")
	}

	embedding := e.impl.Compute(stream)
	return embedding, nil
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

	log.Println("[voiceprint] 声纹提取器已关闭")
}
