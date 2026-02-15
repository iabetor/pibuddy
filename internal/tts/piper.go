package tts

import (
	"bytes"
	"context"
	"fmt"
	"github.com/iabetor/pibuddy/internal/logger"
	"os/exec"

	"github.com/iabetor/pibuddy/internal/audio"
)

// piperSampleRate 是 piper 输出的固定采样率。
const piperSampleRate = 22050

// PiperEngine 使用 piper CLI 子进程实现语音合成，作为离线备用方案。
type PiperEngine struct {
	modelPath string
}

// NewPiperEngine 创建指定模型的 Piper TTS 引擎。
func NewPiperEngine(modelPath string) *PiperEngine {
	return &PiperEngine{modelPath: modelPath}
}

// Synthesize 使用 piper CLI 将文本转换为单声道 float32 音频样本。
// piper 输出 signed 16-bit LE 单声道 PCM，采样率 22050 Hz。
func (p *PiperEngine) Synthesize(ctx context.Context, text string) ([]float32, int, error) {
	logger.Debugf("[tts] piper: 正在合成 %d 个字符，模型=%s", len([]rune(text)), p.modelPath)

	cmd := exec.CommandContext(ctx, "piper", "--model", p.modelPath, "--output-raw")
	cmd.Stdin = bytes.NewReader([]byte(text))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			logger.Warnf("[tts] piper stderr: %s", stderrStr)
		}
		return nil, 0, fmt.Errorf("[tts] piper 执行失败: %w", err)
	}

	pcmData := stdout.Bytes()

	if len(pcmData) == 0 {
		return nil, 0, fmt.Errorf("[tts] piper: 未收到音频数据")
	}

	logger.Debugf("[tts] piper: 收到 %d 字节原始 PCM", len(pcmData))

	// 将 signed 16-bit LE 单声道 PCM 字节转换为 float32 样本
	samples := audio.BytesToFloat32(pcmData)

	logger.Debugf("[tts] piper: 生成 %d 个单声道 float32 样本", len(samples))

	return samples, piperSampleRate, nil
}
