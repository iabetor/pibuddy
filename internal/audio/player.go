package audio

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/gen2brain/malgo"
)

// Player 使用 malgo (miniaudio) 管理音频播放。
type Player struct {
	ctx      *malgo.AllocatedContext
	channels uint32
	mu       sync.Mutex
	closed   bool
}

// NewPlayer 创建一个新的音频播放实例。
// channels: 声道数，通常为 1（单声道）
func NewPlayer(channels int) (*Player, error) {
	ctxConfig := malgo.ContextConfig{}
	ctx, err := malgo.InitContext(nil, ctxConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("初始化播放上下文失败: %w", err)
	}

	return &Player{
		ctx:      ctx,
		channels: uint32(channels),
	}, nil
}

// Play 通过默认扬声器播放 float32 音频样本。
// sampleRate 参数指定音频数据的采样率，播放设备将按此采样率播放。
// 阻塞直到播放完成或 ctx 被取消。
func (p *Player) Play(ctx context.Context, samples []float32, sampleRate int) error {
	if len(samples) == 0 {
		return nil
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return fmt.Errorf("播放器已关闭")
	}
	p.mu.Unlock()

	pcmBytes := Float32ToBytes(samples)
	pos := 0
	done := make(chan struct{})

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = p.channels
	deviceConfig.SampleRate = uint32(sampleRate) // 使用音频实际采样率
	deviceConfig.PeriodSizeInFrames = 512
	deviceConfig.Periods = 2

	callbacks := malgo.DeviceCallbacks{
		Data: func(outputSamples, inputSamples []byte, frameCount uint32) {
			bytesNeeded := int(frameCount) * int(p.channels) * 2 // 每个 int16 采样点 2 字节
			if pos >= len(pcmBytes) {
				// 数据播完，填充静音
				for i := range outputSamples[:bytesNeeded] {
					outputSamples[i] = 0
				}
				select {
				case done <- struct{}{}:
				default:
				}
				return
			}

			end := pos + bytesNeeded
			if end > len(pcmBytes) {
				end = len(pcmBytes)
			}
			copy(outputSamples, pcmBytes[pos:end])
			// 如果数据不够，剩余部分填零
			if end-pos < bytesNeeded {
				for i := end - pos; i < bytesNeeded; i++ {
					outputSamples[i] = 0
				}
			}
			pos = end
		},
	}

	device, err := malgo.InitDevice(p.ctx.Context, deviceConfig, callbacks)
	if err != nil {
		return fmt.Errorf("初始化播放设备失败: %w", err)
	}
	defer device.Uninit()

	if err := device.Start(); err != nil {
		return fmt.Errorf("启动播放设备失败: %w", err)
	}
	defer device.Stop()

	select {
	case <-ctx.Done():
		log.Println("[audio] 播放被取消")
		return ctx.Err()
	case <-done:
		log.Println("[audio] 播放完成")
		return nil
	}
}

// Close 释放所有资源。
func (p *Player) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}
	p.closed = true

	if p.ctx != nil {
		_ = p.ctx.Uninit()
		p.ctx.Free()
		p.ctx = nil
	}
}
