package tts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/iabetor/pibuddy/internal/audio"
	"github.com/iabetor/pibuddy/internal/logger"
)

// saySampleRate 是 macOS say 输出的采样率。
const saySampleRate = 22050

// SayEngine 使用 macOS 内置 say 命令实现语音合成，作为离线备用方案。
// 仅在 macOS 上可用。
type SayEngine struct {
	voice string // macOS 语音名称，如 "Tingting"（中文）
}

// NewSayEngine 创建 macOS say TTS 引擎。
// voice 为空时使用系统默认语音。
func NewSayEngine(voice string) *SayEngine {
	return &SayEngine{voice: voice}
}

// Synthesize 使用 macOS say 命令将文本转换为单声道 float32 音频样本。
// say 先输出 AIFF 文件，再用 afconvert 转为 16-bit LE PCM。
func (s *SayEngine) Synthesize(ctx context.Context, text string) ([]float32, int, error) {
	logger.Debugf("[tts] say: 正在合成 %d 个字符", len([]rune(text)))

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "pibuddy-say-*.aiff")
	if err != nil {
		return nil, 0, fmt.Errorf("[tts] say: 创建临时文件失败: %w", err)
	}
	aiffPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(aiffPath)

	rawPath := aiffPath + ".raw"
	defer os.Remove(rawPath)

	// 使用 say 命令生成 AIFF 文件
	args := []string{"-o", aiffPath}
	if s.voice != "" {
		args = append(args, "-v", s.voice)
	}
	args = append(args, text)

	cmd := exec.CommandContext(ctx, "say", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, 0, fmt.Errorf("[tts] say 执行失败: %w, stderr: %s", err, stderr.String())
	}

	// 使用 afconvert 转换为 16-bit LE 单声道 PCM
	convertCmd := exec.CommandContext(ctx, "afconvert",
		"-f", "WAVE",
		"-d", "LEI16@22050",
		"-c", "1",
		aiffPath, rawPath,
	)
	var convertStderr bytes.Buffer
	convertCmd.Stderr = &convertStderr

	if err := convertCmd.Run(); err != nil {
		return nil, 0, fmt.Errorf("[tts] afconvert 执行失败: %w, stderr: %s", err, convertStderr.String())
	}

	// 读取 WAV 文件，跳过 44 字节的 WAV header
	wavData, err := os.ReadFile(rawPath)
	if err != nil {
		return nil, 0, fmt.Errorf("[tts] say: 读取输出文件失败: %w", err)
	}

	if len(wavData) <= 44 {
		return nil, 0, fmt.Errorf("[tts] say: 未收到音频数据")
	}

	// 跳过 WAV header（44 字节）
	pcmData := wavData[44:]

	logger.Debugf("[tts] say: 收到 %d 字节原始 PCM", len(pcmData))

	samples := audio.BytesToFloat32(pcmData)

	logger.Debugf("[tts] say: 生成 %d 个单声道 float32 样本", len(samples))

	return samples, saySampleRate, nil
}
