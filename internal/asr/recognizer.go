package asr

import (
	"fmt"
	"github.com/iabetor/pibuddy/internal/logger"
	"path/filepath"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// Recognizer 封装 sherpa-onnx 流式在线语音识别器（Zipformer），
// 维护一个持久化的 OnlineStream 用于增量接收音频并输出识别结果。
type Recognizer struct {
	recognizer *sherpa.OnlineRecognizer
	stream     *sherpa.OnlineStream
}

// NewRecognizer 创建流式语音识别器。
// modelPath: 包含 Zipformer 模型文件（encoder、decoder、joiner ONNX 文件和 tokens.txt）的目录
// numThreads: 推理引擎使用的 CPU 线程数
func NewRecognizer(modelPath string, numThreads int) (*Recognizer, error) {
	config := sherpa.OnlineRecognizerConfig{}

	// 特征提取配置
	config.FeatConfig.SampleRate = 16000
	config.FeatConfig.FeatureDim = 80

	// Transducer 模型路径（流式双语 Zipformer 的常见文件命名）
	config.ModelConfig.Transducer.Encoder = filepath.Join(modelPath, "encoder-epoch-99-avg-1.onnx")
	config.ModelConfig.Transducer.Decoder = filepath.Join(modelPath, "decoder-epoch-99-avg-1.onnx")
	config.ModelConfig.Transducer.Joiner = filepath.Join(modelPath, "joiner-epoch-99-avg-1.onnx")

	// 词表和运行时配置
	config.ModelConfig.Tokens = filepath.Join(modelPath, "tokens.txt")
	config.ModelConfig.NumThreads = numThreads
	config.ModelConfig.Provider = "cpu"
	config.ModelConfig.ModelType = "zipformer"

	// 解码设置
	config.DecodingMethod = "greedy_search"

	// 端点检测设置
	config.EnableEndpoint = 1
	config.Rule1MinTrailingSilence = 2.4
	config.Rule2MinTrailingSilence = 1.2
	config.Rule3MinUtteranceLength = 20.0

	recognizer := sherpa.NewOnlineRecognizer(&config)
	if recognizer == nil {
		return nil, fmt.Errorf("创建在线识别器失败，模型路径: %s", modelPath)
	}

	stream := sherpa.NewOnlineStream(recognizer)
	if stream == nil {
		sherpa.DeleteOnlineRecognizer(recognizer)
		return nil, fmt.Errorf("创建在线识别流失败")
	}

	logger.Infof("[asr] 语音识别器已初始化 (model=%s, threads=%d)", modelPath, numThreads)

	return &Recognizer{
		recognizer: recognizer,
		stream:     stream,
	}, nil
}

// Feed 将音频样本送入识别流，并立即解码一帧。
// 样本应为 16kHz float32 格式。
// 注意：Feed 后立即调用 Decode，减少 circular buffer 积压，避免 Overflow 警告。
func (r *Recognizer) Feed(samples []float32) {
	r.stream.AcceptWaveform(16000, samples)
	// 立即解码一帧，减少 buffer 积压
	if r.recognizer.IsReady(r.stream) {
		r.recognizer.Decode(r.stream)
	}
}

// IsEndpoint 返回识别器是否检测到端点（即说话者已结束一句话）。
func (r *Recognizer) IsEndpoint() bool {
	return r.recognizer.IsEndpoint(r.stream)
}

// GetResult 解码所有待处理帧并返回当前识别文本。
// 循环调用 Decode 直到没有待处理帧，防止 circular buffer 因积压而 Overflow。
// 如果还没有识别到任何内容，返回空字符串。
func (r *Recognizer) GetResult() string {
	for r.recognizer.IsReady(r.stream) {
		r.recognizer.Decode(r.stream)
	}
	result := r.recognizer.GetResult(r.stream)
	return result.Text
}

// Reset 重置识别流状态，为处理新的语句做准备。
// 在获取完一个端点的结果后应调用此方法。
// 通过销毁旧 stream 并创建新 stream 来彻底清空内部 circular buffer，
// 避免长时间运行后 sherpa-onnx circular-buffer Overflow 警告。
func (r *Recognizer) Reset() {
	if r.recognizer != nil && r.stream != nil {
		sherpa.DeleteOnlineStream(r.stream)
		r.stream = sherpa.NewOnlineStream(r.recognizer)
	}
}

// Close 释放底层 sherpa-onnx 资源。调用后不可再使用此 Recognizer。
func (r *Recognizer) Close() {
	if r.stream != nil {
		sherpa.DeleteOnlineStream(r.stream)
		r.stream = nil
	}
	if r.recognizer != nil {
		sherpa.DeleteOnlineRecognizer(r.recognizer)
		r.recognizer = nil
	}
	logger.Info("[asr] 语音识别器已关闭")
}
