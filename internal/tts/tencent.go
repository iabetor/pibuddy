package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/google/uuid"
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
	speed     float64
}

// TencentConfig 腾讯云 TTS 配置。
type TencentConfig struct {
	SecretID  string
	SecretKey string
	VoiceType int64
	Region    string
	Speed     float64
}

// NewTencentEngine 创建腾讯云 TTS 引擎。
func NewTencentEngine(cfg TencentConfig) (*TencentEngine, error) {
	// 调试：打印实际收到的 SecretID 前缀，确认环境变量展开正确
	if len(cfg.SecretID) > 8 {
		log.Printf("[tts] 腾讯云 TTS: SecretID=%s..., SecretKey长度=%d", cfg.SecretID[:8], len(cfg.SecretKey))
	} else {
		log.Printf("[tts] 腾讯云 TTS: SecretID='%s' (长度=%d), SecretKey长度=%d", cfg.SecretID, len(cfg.SecretID), len(cfg.SecretKey))
	}

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

	log.Printf("[tts] 腾讯云 TTS 引擎已初始化 (voice=%d, region=%s, speed=%.1f)", cfg.VoiceType, cfg.Region, cfg.Speed)

	return &TencentEngine{
		client:    client,
		voiceType: cfg.VoiceType,
		speed:     cfg.Speed,
	}, nil
}

// reHanOrLetter 匹配至少包含一个中文字符或字母的文本。
var reHanOrLetter = regexp.MustCompile(`[\p{Han}a-zA-Z]`)

// sanitizeText 清理文本，移除 emoji 和不可合成的字符，
// 仅保留中日韩文字、字母、数字和常见标点。
func sanitizeText(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch {
		case r >= 0x4E00 && r <= 0x9FFF: // CJK 统一汉字
			b.WriteRune(r)
		case r >= 0x3400 && r <= 0x4DBF: // CJK 扩展 A
			b.WriteRune(r)
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case isPunct(r):
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// isPunct 判断是否为常见中英文标点或空白符。
func isPunct(r rune) bool {
	puncts := "，。！？、；：（）《》【】—…·,.!?;:'-/ \t\n"
	return strings.ContainsRune(puncts, r) ||
		r == '\u201C' || r == '\u201D' || // ""
		r == '\u2018' || r == '\u2019' || // ''
		r == '"'
}

// Synthesize 将文本合成为单声道 float32 音频样本。
// 腾讯云 TTS 返回 MP3 格式，需要解码为 PCM。
func (e *TencentEngine) Synthesize(ctx context.Context, text string) ([]float32, int, error) {
	// 清理文本，移除 emoji 等不可合成字符
	cleaned := sanitizeText(text)
	if !reHanOrLetter.MatchString(cleaned) {
		log.Printf("[tts] 腾讯云 TTS: 跳过无有效文字的文本: %q", text)
		return nil, 0, nil
	}

	log.Printf("[tts] 腾讯云 TTS: 正在合成 %d 个字符，音色=%d", len([]rune(cleaned)), e.voiceType)

	request := tts.NewTextToVoiceRequest()
	request.Text = common.StringPtr(cleaned)
	request.SessionId = common.StringPtr(uuid.New().String())
	request.VoiceType = common.Int64Ptr(e.voiceType)
	request.Codec = common.StringPtr("mp3")
	request.Speed = common.Float64Ptr(e.speed)
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
