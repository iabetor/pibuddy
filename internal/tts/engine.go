package tts

import "context"

// Engine 定义语音合成后端接口。
type Engine interface {
	// Synthesize 将文本转换为音频。
	// 返回 float32 音频样本、采样率（Hz）和错误。
	Synthesize(ctx context.Context, text string) ([]float32, int, error)
}
