package tts

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"

	"github.com/hajimehoshi/go-mp3"
	"github.com/pp-group/edge-tts-go/biz/service/tts/edge"
)

// EdgeEngine 使用微软 Edge TTS 实现语音合成，
// 通过 edge-tts-go 获取 MP3 音频，再用 go-mp3 解码为 PCM。
type EdgeEngine struct {
	voice string
}

// NewEdgeEngine 创建指定语音的 Edge TTS 引擎。
func NewEdgeEngine(voice string) *EdgeEngine {
	return &EdgeEngine{voice: voice}
}

// Synthesize 将文本合成为单声道 float32 音频样本。
// 返回样本数据、采样率和错误。
func (e *EdgeEngine) Synthesize(ctx context.Context, text string) ([]float32, int, error) {
	log.Printf("[tts] edge-tts: 正在合成 %d 个字符，语音=%s", len([]rune(text)), e.voice)

	// 创建 Communicate 实例并通过 Stream() 获取 MP3 音频块
	comm, err := edge.NewCommunicate(text, edge.WithVoice(e.voice))
	if err != nil {
		return nil, 0, fmt.Errorf("[tts] edge-tts 创建实例失败: %w", err)
	}

	ch, err := comm.Stream()
	if err != nil {
		return nil, 0, fmt.Errorf("[tts] edge-tts 开始流式合成失败: %w", err)
	}

	// 从 channel 收集所有音频数据
	var mp3Buf bytes.Buffer
	for msg := range ch {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		default:
		}
		// Stream() 返回的 map 中，type=="audio" 的条目包含音频数据
		if msgType, ok := msg["type"].(string); ok && msgType == "audio" {
			if data, ok := msg["data"].([]byte); ok {
				mp3Buf.Write(data)
			}
		}
	}

	mp3Data := mp3Buf.Bytes()
	if len(mp3Data) == 0 {
		return nil, 0, fmt.Errorf("[tts] edge-tts: 未收到音频数据")
	}

	log.Printf("[tts] edge-tts: 收到 %d 字节 MP3 数据", len(mp3Data))

	// 解码 MP3 为原始 PCM
	decoder, err := mp3.NewDecoder(bytes.NewReader(mp3Data))
	if err != nil {
		return nil, 0, fmt.Errorf("[tts] MP3 解码失败: %w", err)
	}

	sampleRate := decoder.SampleRate()

	pcmData, err := io.ReadAll(decoder)
	if err != nil {
		return nil, 0, fmt.Errorf("[tts] 读取 PCM 数据失败: %w", err)
	}

	log.Printf("[tts] edge-tts: 解码得到 %d 字节 PCM，采样率 %d Hz", len(pcmData), sampleRate)

	// 将立体声 signed 16-bit LE PCM 转换为单声道 float32
	// 每个立体声帧 4 字节：左声道 2 字节 + 右声道 2 字节
	const bytesPerFrame = 4
	if len(pcmData)%bytesPerFrame != 0 {
		// 截掉不完整的尾部帧
		pcmData = pcmData[:len(pcmData)/bytesPerFrame*bytesPerFrame]
	}

	numFrames := len(pcmData) / bytesPerFrame
	samples := make([]float32, numFrames)

	for i := 0; i < numFrames; i++ {
		offset := i * bytesPerFrame
		left := int16(binary.LittleEndian.Uint16(pcmData[offset : offset+2]))
		right := int16(binary.LittleEndian.Uint16(pcmData[offset+2 : offset+4]))

		// 左右声道取平均得到单声道，归一化到 [-1.0, 1.0]
		mono := (float32(left) + float32(right)) / 2.0
		samples[i] = mono / 32768.0
	}

	log.Printf("[tts] edge-tts: 生成 %d 个单声道 float32 样本", len(samples))

	return samples, sampleRate, nil
}
