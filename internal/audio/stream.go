package audio

import (
	"context"
	"fmt"
	"io"
	"github.com/iabetor/pibuddy/internal/logger"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gen2brain/malgo"
	"github.com/hajimehoshi/go-mp3"
)

// StreamPlayer 支持从 HTTP URL 流式播放 MP3 音频。
type StreamPlayer struct {
	ctx      *malgo.AllocatedContext
	channels uint32
	mu       sync.Mutex
	cancel   context.CancelFunc
	closed   bool
}

// NewStreamPlayer 创建流式播放器。
func NewStreamPlayer(channels int) (*StreamPlayer, error) {
	ctxConfig := malgo.ContextConfig{}
	ctx, err := malgo.InitContext(nil, ctxConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("初始化播放上下文失败: %w", err)
	}

	return &StreamPlayer{
		ctx:      ctx,
		channels: uint32(channels),
	}, nil
}

// Play 从 URL 流式下载并播放 MP3 音频。
func (sp *StreamPlayer) Play(ctx context.Context, url string) error {
	sp.mu.Lock()
	if sp.closed {
		sp.mu.Unlock()
		return fmt.Errorf("播放器已关闭")
	}
	streamCtx, cancel := context.WithCancel(ctx)
	sp.cancel = cancel
	sp.mu.Unlock()

	defer func() {
		sp.mu.Lock()
		sp.cancel = nil
		sp.mu.Unlock()
	}()

	// 下载音频
	req, err := http.NewRequestWithContext(streamCtx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("下载音频失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载音频返回错误状态码: %d", resp.StatusCode)
	}

	// 解码 MP3
	decoder, err := mp3.NewDecoder(resp.Body)
	if err != nil {
		return fmt.Errorf("创建 MP3 解码器失败: %w", err)
	}

	sampleRate := decoder.SampleRate()
	logger.Debugf("[audio] 流式播放: 采样率 %d Hz", sampleRate)

	// 创建音频数据通道，根据采样率动态计算块大小
	chunkSize := sampleRate * 2 // 约 2 秒的样本数
	const bufferChunks = 5
	sampleCh := make(chan []float32, bufferChunks)
	errCh := make(chan error, 1)

	// 生产者：后台解码
	go func() {
		defer close(sampleCh)

		buf := make([]byte, 16384) // 更大的读取缓冲区
		var samples []float32

		for {
			select {
			case <-streamCtx.Done():
				return
			default:
			}

			n, err := decoder.Read(buf)
			if err != nil {
				// EOF 或网络错误都视为播放结束
				if err == io.EOF || isNetworkError(err) {
					if len(samples) > 0 {
						select {
						case sampleCh <- samples:
						case <-streamCtx.Done():
						}
					}
					logger.Debugf("[audio] 解码结束: %v", err)
					return
				}
				select {
				case errCh <- fmt.Errorf("读取音频数据失败: %w", err):
				default:
				}
				return
			}

			if n == 0 {
				continue
			}

			chunkSamples := int16StereoToMonoFloat32(buf[:n])
			samples = append(samples, chunkSamples...)

			for len(samples) >= chunkSize {
				chunk := make([]float32, chunkSize)
				copy(chunk, samples[:chunkSize])
				samples = samples[chunkSize:]

				select {
				case sampleCh <- chunk:
				case <-streamCtx.Done():
					return
				}
			}
		}
	}()

	// 预缓冲：等待至少 2 块数据
	preBuffer := make([][]float32, 0, 2)
preBufferLoop:
	for len(preBuffer) < 2 {
		select {
		case <-streamCtx.Done():
			return streamCtx.Err()
		case err := <-errCh:
			return err
		case chunk, ok := <-sampleCh:
			if !ok {
				break preBufferLoop
			}
			preBuffer = append(preBuffer, chunk)
			logger.Debugf("[audio] 预缓冲 %d/2", len(preBuffer))
		}
	}
	if len(preBuffer) == 0 {
		return nil // 空文件
	}
	logger.Debugf("[audio] 预缓冲完成，开始播放")

	// 合并预缓冲数据
	var totalLen int
	for _, c := range preBuffer {
		totalLen += len(c)
	}
	pcmData := make([]byte, 0, totalLen*2)
	for _, c := range preBuffer {
		pcmData = append(pcmData, Float32ToBytes(c)...)
	}
	pos := 0
	done := make(chan struct{})

	// 配置播放设备
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = sp.channels
	deviceConfig.SampleRate = uint32(sampleRate)
	deviceConfig.PeriodSizeInFrames = 4096 // 更大的缓冲区
	deviceConfig.Periods = 4

	callbacks := malgo.DeviceCallbacks{
		Data: func(outputSamples, inputSamples []byte, frameCount uint32) {
			bytesNeeded := int(frameCount) * int(sp.channels) * 2

			for bytesNeeded > 0 {
				if pos >= len(pcmData) {
					// 当前块播完，尝试获取下一块
					select {
					case chunk, ok := <-sampleCh:
						if !ok {
							// 所有数据播完
							for i := range outputSamples[:bytesNeeded] {
								outputSamples[i] = 0
							}
							select {
							case done <- struct{}{}:
							default:
							}
							return
						}
						pcmData = Float32ToBytes(chunk)
						pos = 0
					default:
						// 通道为空，填充静音等待
						for i := range outputSamples[:bytesNeeded] {
							outputSamples[i] = 0
						}
						return
					}
				}

				end := pos + bytesNeeded
				if end > len(pcmData) {
					end = len(pcmData)
				}
				copied := copy(outputSamples[len(outputSamples)-bytesNeeded:], pcmData[pos:end])
				pos = end
				bytesNeeded -= copied
			}
		},
	}

	device, err := malgo.InitDevice(sp.ctx.Context, deviceConfig, callbacks)
	if err != nil {
		return fmt.Errorf("初始化播放设备失败: %w", err)
	}
	defer device.Uninit()

	if err := device.Start(); err != nil {
		return fmt.Errorf("启动播放设备失败: %w", err)
	}
	defer device.Stop()

	select {
	case <-streamCtx.Done():
		logger.Debug("[audio] 流式播放被取消")
		return streamCtx.Err()
	case err := <-errCh:
		return err
	case <-done:
		logger.Debug("[audio] 流式播放完成")
		return nil
	}
}

// Stop 停止当前播放。
func (sp *StreamPlayer) Stop() {
	sp.mu.Lock()
	if sp.cancel != nil {
		sp.cancel()
	}
	sp.mu.Unlock()
}

// Close 释放资源。
func (sp *StreamPlayer) Close() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if sp.closed {
		return
	}
	sp.closed = true

	if sp.ctx != nil {
		_ = sp.ctx.Uninit()
		sp.ctx.Free()
		sp.ctx = nil
	}
}

// int16StereoToMonoFloat32 将 int16 立体声 PCM 转换为单声道 float32。
func int16StereoToMonoFloat32(data []byte) []float32 {
	numSamples := len(data) / 4
	if numSamples == 0 {
		return nil
	}
	samples := make([]float32, numSamples)

	for i := 0; i < numSamples; i++ {
		left := int16(data[i*4]) | int16(data[i*4+1])<<8
		right := int16(data[i*4+2]) | int16(data[i*4+3])<<8
		samples[i] = (float32(left) + float32(right)) / 65536.0
	}

	return samples
}

// isNetworkError 判断是否为网络错误（连接断开等）。
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// 连接被重置、连接断开等
	if netErr, ok := err.(net.Error); ok {
		return strings.Contains(netErr.Error(), "connection reset") ||
			strings.Contains(netErr.Error(), "broken pipe") ||
			strings.Contains(netErr.Error(), "connection refused")
	}
	return strings.Contains(err.Error(), "connection reset by peer") ||
		strings.Contains(err.Error(), "broken pipe")
}
