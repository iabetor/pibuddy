package tts

import (
	"context"
	"strings"
)

// Engine 定义语音合成后端接口。
type Engine interface {
	// Synthesize 将文本转换为音频。
	// 返回 float32 音频样本、采样率（Hz）和错误。
	Synthesize(ctx context.Context, text string) ([]float32, int, error)
}

// PreprocessText 预处理文本，删除不适合朗读的字符。
// 所有 TTS 引擎调用前应先使用此函数处理文本。
func PreprocessText(text string) string {
	// 删除 Markdown 格式符号
	text = strings.ReplaceAll(text, "**", "")  // 粗体
	text = strings.ReplaceAll(text, "__", "")  // 粗体
	text = strings.ReplaceAll(text, "*", "")   // 斜体
	text = strings.ReplaceAll(text, "_", "")   // 斜体下划线
	text = strings.ReplaceAll(text, "`", "")   // 代码
	text = strings.ReplaceAll(text, "~~", "")  // 删除线
	text = strings.ReplaceAll(text, "#", "")   // 标题

	// 删除省略号（中文和英文）
	text = strings.ReplaceAll(text, "……", "")
	text = strings.ReplaceAll(text, "...", "")
	text = strings.ReplaceAll(text, "…", "")

	// 清理多余的空格
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	return strings.TrimSpace(text)
}
