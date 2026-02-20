package asr

import (
	"fmt"
	"path/filepath"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
	"github.com/iabetor/pibuddy/internal/logger"
)

// SherpaEngine 封装 sherpa-onnx 流式在线语音识别器（Zipformer），
// 维护一个持久化的 OnlineStream 用于增量接收音频并输出识别结果。
// 实现 Engine 接口。
type SherpaEngine struct {
	recognizer *sherpa.OnlineRecognizer
	stream     *sherpa.OnlineStream
}

// 确保实现 Engine 接口
var _ Engine = (*SherpaEngine)(nil)

// Recognizer 是 SherpaEngine 的别名，保持向后兼容。
// Deprecated: 使用 SherpaEngine 代替。
type Recognizer = SherpaEngine

// NewSherpaEngine 创建 sherpa-onnx 流式语音识别器。
// modelPath: 包含 Zipformer 模型文件（encoder、decoder、joiner ONNX 文件和 tokens.txt）的目录
// numThreads: 推理引擎使用的 CPU 线程数
// rule1MinTrailingSilence: 尾部静音阈值（秒），默认 2.4
// rule2MinTrailingSilence: 尾部静音阈值（秒），默认 1.2
// rule3MinUtteranceLength: 最小语音长度（秒），默认 20.0
func NewSherpaEngine(modelPath string, numThreads int, rule1MinTrailingSilence, rule2MinTrailingSilence, rule3MinUtteranceLength float64) (*SherpaEngine, error) {
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
	// 使用传入参数，如果为 0 则使用默认值
	if rule1MinTrailingSilence > 0 {
		config.Rule1MinTrailingSilence = float32(rule1MinTrailingSilence)
	} else {
		config.Rule1MinTrailingSilence = 2.4
	}
	if rule2MinTrailingSilence > 0 {
		config.Rule2MinTrailingSilence = float32(rule2MinTrailingSilence)
	} else {
		config.Rule2MinTrailingSilence = 1.2
	}
	if rule3MinUtteranceLength > 0 {
		config.Rule3MinUtteranceLength = float32(rule3MinUtteranceLength)
	} else {
		config.Rule3MinUtteranceLength = 20.0
	}

	recognizer := sherpa.NewOnlineRecognizer(&config)
	if recognizer == nil {
		return nil, fmt.Errorf("创建在线识别器失败，模型路径: %s", modelPath)
	}

	stream := sherpa.NewOnlineStream(recognizer)
	if stream == nil {
		sherpa.DeleteOnlineRecognizer(recognizer)
		return nil, fmt.Errorf("创建在线识别流失败")
	}

	logger.Infof("[asr] Sherpa 引擎已初始化 (model=%s, threads=%d)", modelPath, numThreads)

	return &SherpaEngine{
		recognizer: recognizer,
		stream:     stream,
	}, nil
}

// Feed 将音频样本送入识别流，并立即解码一帧。
// 样本应为 16kHz float32 格式。
// 注意：Feed 后立即调用 Decode，减少 circular buffer 积压，避免 Overflow 警告。
func (e *SherpaEngine) Feed(samples []float32) {
	e.stream.AcceptWaveform(16000, samples)
	// 立即解码一帧，减少 buffer 积压
	if e.recognizer.IsReady(e.stream) {
		e.recognizer.Decode(e.stream)
	}
}

// IsEndpoint 返回识别器是否检测到端点（即说话者已结束一句话）。
func (e *SherpaEngine) IsEndpoint() bool {
	return e.recognizer.IsEndpoint(e.stream)
}

// GetResult 解码所有待处理帧并返回当前识别文本。
// 循环调用 Decode 直到没有待处理帧，防止 circular buffer 因积压而 Overflow。
// 如果还没有识别到任何内容，返回空字符串。
func (e *SherpaEngine) GetResult() string {
	for e.recognizer.IsReady(e.stream) {
		e.recognizer.Decode(e.stream)
	}
	result := e.recognizer.GetResult(e.stream)
	return result.Text
}

// Reset 重置识别流状态，为处理新的语句做准备。
// 在获取完一个端点的结果后应调用此方法。
// 通过销毁旧 stream 并创建新 stream 来彻底清空内部 circular buffer，
// 避免长时间运行后 sherpa-onnx circular-buffer Overflow 警告。
func (e *SherpaEngine) Reset() {
	if e.recognizer != nil && e.stream != nil {
		sherpa.DeleteOnlineStream(e.stream)
		e.stream = sherpa.NewOnlineStream(e.recognizer)
	}
}

// Close 释放底层 sherpa-onnx 资源。调用后不可再使用此引擎。
func (e *SherpaEngine) Close() {
	if e.stream != nil {
		sherpa.DeleteOnlineStream(e.stream)
		e.stream = nil
	}
	if e.recognizer != nil {
		sherpa.DeleteOnlineRecognizer(e.recognizer)
		e.recognizer = nil
	}
	logger.Info("[asr] Sherpa 引擎已关闭")
}

// Name 返回引擎名称。
func (e *SherpaEngine) Name() string {
	return string(EngineSherpa)
}

// NewRecognizer 是 NewSherpaEngine 的别名，保持向后兼容。
// Deprecated: 使用 NewSherpaEngine 代替。
func NewRecognizer(modelPath string, numThreads int, rule1MinTrailingSilence, rule2MinTrailingSilence, rule3MinUtteranceLength float64) (*SherpaEngine, error) {
	return NewSherpaEngine(modelPath, numThreads, rule1MinTrailingSilence, rule2MinTrailingSilence, rule3MinUtteranceLength)
}
