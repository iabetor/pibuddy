package audio

import (
	"context"
	"fmt"
	"github.com/iabetor/pibuddy/internal/logger"
	"sync"

	"github.com/gen2brain/malgo"
)

// Capture 使用 malgo (miniaudio) 管理麦克风音频采集。
// 采集到的音频以 float32 帧的形式发送到输出 channel。
type Capture struct {
	ctx        *malgo.AllocatedContext
	device     *malgo.Device
	sampleRate uint32
	channels   uint32
	frameSize  uint32
	out        chan []float32
	mu         sync.Mutex
	running    bool
}

// NewCapture 创建一个新的音频采集实例。
// sampleRate: 采样率，语音处理通常用 16000
// channels: 声道数，通常为 1（单声道）
// frameSize: 每帧的采样点数（如 512）
func NewCapture(sampleRate, channels, frameSize int) (*Capture, error) {
	ctxConfig := malgo.ContextConfig{}
	ctxConfig.ThreadPriority = malgo.ThreadPriorityRealtime

	ctx, err := malgo.InitContext(nil, ctxConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("初始化音频上下文失败: %w", err)
	}

	return &Capture{
		ctx:        ctx,
		sampleRate: uint32(sampleRate),
		channels:   uint32(channels),
		frameSize:  uint32(frameSize),
		out:        make(chan []float32, 64),
	}, nil
}

// C 返回接收音频帧的只读 channel。
func (c *Capture) C() <-chan []float32 {
	return c.out
}

// Start 开始从默认麦克风采集音频。
func (c *Capture) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = c.channels
	deviceConfig.SampleRate = c.sampleRate
	deviceConfig.PeriodSizeInFrames = c.frameSize
	deviceConfig.Periods = 2

	callbacks := malgo.DeviceCallbacks{
		Data: func(outputSamples, inputSamples []byte, frameCount uint32) {
			if len(inputSamples) == 0 {
				return
			}
			samples := BytesToFloat32(inputSamples)
			// 非阻塞发送 —— 如果消费端跟不上就丢帧
			select {
			case c.out <- samples:
			default:
			}
		},
	}

	device, err := malgo.InitDevice(c.ctx.Context, deviceConfig, callbacks)
	if err != nil {
		return fmt.Errorf("初始化采集设备失败: %w", err)
	}

	if err := device.Start(); err != nil {
		device.Uninit()
		return fmt.Errorf("启动采集设备失败: %w", err)
	}

	c.device = device
	c.running = true
	logger.Info("[audio] 麦克风采集已启动")
	return nil
}

// Stop 停止音频采集。
func (c *Capture) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	c.device.Stop()
	c.device.Uninit()
	c.running = false
	logger.Info("[audio] 麦克风采集已停止")
}

// Drain 清空采集 channel 中的残留音频帧。
// 在 Speaking → Listening 转换时调用，避免扬声器回声被 ASR 识别。
func (c *Capture) Drain() int {
	n := 0
	for {
		select {
		case <-c.out:
			n++
		default:
			if n > 0 {
				logger.Debugf("[audio] 清空麦克风缓冲: 丢弃 %d 帧", n)
			}
			return n
		}
	}
}

// Close 释放所有资源。
func (c *Capture) Close() {
	c.Stop()
	if c.ctx != nil {
		_ = c.ctx.Uninit()
		c.ctx.Free()
	}
	close(c.out)
}

// RecordFor 持续录音直到 ctx 取消，返回所有采样点。用于测试。
func (c *Capture) RecordFor(ctx context.Context) []float32 {
	var all []float32
	for {
		select {
		case <-ctx.Done():
			return all
		case frame, ok := <-c.out:
			if !ok {
				return all
			}
			all = append(all, frame...)
		}
	}
}
