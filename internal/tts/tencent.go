package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"

	"github.com/hajimehoshi/go-mp3"
	tts "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tts/v20190823"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
)

// TencentEngine 使用腾讯云 TTS 实现语音合成。
// 适用于中国大陆网络环境，支持多种中文音色。
type TencentEngine struct {
	client    *tts.Client
	voiceType int64
}

// TencentConfig 腾讯云 TTS 配置。
type TencentConfig struct {
	SecretID  string
	SecretKey string
	VoiceType int64
	Region    string
}

// NewTencentEngine 创建腾讯云 TTS 引擎。
func NewTencentEngine(cfg TencentConfig) (*TencentEngine, error) {
	if cfg.SecretID == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("[tts] 腾讯云 TTS 需要 SecretID 和 SecretKey")
	}

	if cfg.VoiceType == 0 {
		cfg.VoiceType = 1001 // 默认音色：智瑜（女声）
	}
	if cfg.Region == "" {
		cfg.Region = "ap-guangzhou"
	}

	credential := common.NewCredential(cfg.SecretID, cfg.SecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "tts.tencentcloudapi.com"

	client, err := tts.NewClient(credential, cfg.Region, cpf)
	if err != nil {
		return nil, fmt.Errorf("[tts] 创建腾讯云 TTS 客户端失败: %w", err)
	}

	log.Printf("[tts] 腾讯云 TTS 引擎已初始化 (voice=%d, region=%s)", cfg.VoiceType, cfg.Region)

	return &TencentEngine{
		client:    client,
		voiceType: cfg.VoiceType,
	}, nil
}

// Synthesize 将文本合成为单声道 float32 音频样本。
// 腾讯云 TTS 返回 MP3 格式，需要解码为 PCM。
func (e *TencentEngine) Synthesize(ctx context.Context, text string) ([]float32, int, error) {
	log.Printf("[tts] 腾讯云 TTS: 正在合成 %d 个字符，音色=%d", len([]rune(text)), e.voiceType)

	request := tts.NewTextToVoiceRequest()
	request.Text = common.StringPtr(text)
	request.VoiceType = common.Int64Ptr(e.voiceType)
	request.Codec = common.StringPtr("mp3")
	request.Speed = common.Float64Ptr(1.0)
	request.Volume = common.Float64Ptr(5.0)

	response, err := e.client.TextToVoice(request)
	if err != nil {
		return nil, 0, fmt.Errorf("[tts] 腾讯云 TTS 合成失败: %w", err)
	}

	if response.Response == nil || response.Response.Audio == nil {
		return nil, 0, fmt.Errorf("[tts] 腾讯云 TTS: 未返回音频数据")
	}

	// Base64 解码
	mp3Data, err := base64.StdEncoding.DecodeString(*response.Response.Audio)
	if err != nil {
		return nil, 0, fmt.Errorf("[tts] Base64 解码失败: %w", err)
	}

	log.Printf("[tts] 腾讯云 TTS: 收到 %d 字节 MP3 数据", len(mp3Data))

	// 解码 MP3 为原始 PCM
	decoder, err := mp3.NewDecoder(bytes.NewReader(mp3Data))
	if err != nil {
		return nil, 0, fmt.Errorf("[tts] MP3 解码失败: %w", err)
	}

	sampleRate := decoder.SampleRate()

	// 读取 PCM 数据
	pcmBuf := new(bytes.Buffer)
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		default:
		}
		n, err := decoder.Read(buf)
		if err != nil {
			break
		}
		pcmBuf.Write(buf[:n])
	}

	pcmData := pcmBuf.Bytes()
	log.Printf("[tts] 腾讯云 TTS: 解码得到 %d 字节 PCM，采样率 %d Hz", len(pcmData), sampleRate)

	// 将立体声 signed 16-bit LE PCM 转换为单声道 float32
	const bytesPerFrame = 4
	if len(pcmData)%bytesPerFrame != 0 {
		pcmData = pcmData[:len(pcmData)/bytesPerFrame*bytesPerFrame]
	}

	numFrames := len(pcmData) / bytesPerFrame
	samples := make([]float32, numFrames)

	for i := 0; i < numFrames; i++ {
		offset := i * bytesPerFrame
		left := int16(binary.LittleEndian.Uint16(pcmData[offset : offset+2]))
		right := int16(binary.LittleEndian.Uint16(pcmData[offset+2 : offset+4]))
		mono := (float32(left) + float32(right)) / 2.0
		samples[i] = mono / 32768.0
	}

	log.Printf("[tts] 腾讯云 TTS: 生成 %d 个单声道 float32 样本", len(samples))

	return samples, sampleRate, nil
}
