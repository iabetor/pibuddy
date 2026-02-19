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
	"time"

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

	// 先完整下载音频到内存缓冲，支持网络中断重试
	audioData, err := sp.downloadWithRetry(streamCtx, url)
	if err != nil {
		return fmt.Errorf("下载音频失败: %w", err)
	}
	if len(audioData) == 0 {
		return nil
	}

	// 解码 MP3
	reader := newBytesReadSeeker(audioData)
	decoder, err := mp3.NewDecoder(reader)
	if err != nil {
		return fmt.Errorf("创建 MP3 解码器失败: %w", err)
	}

	sampleRate := decoder.SampleRate()
	logger.Debugf("[audio] 流式播放: 采样率 %d Hz, 数据 %d 字节", sampleRate, len(audioData))

	// 创建音频数据通道
	chunkSize := sampleRate * 2 // 约 2 秒的样本数
	const bufferChunks = 5
	sampleCh := make(chan []float32, bufferChunks)
	errCh := make(chan error, 1)

	// 生产者：后台解码（数据已在内存中，不会有网络错误）
	go func() {
		defer close(sampleCh)

		buf := make([]byte, 16384)
		var samples []float32

		for {
			select {
			case <-streamCtx.Done():
				return
			default:
			}

			n, err := decoder.Read(buf)
			if err != nil {
				if err == io.EOF {
					if len(samples) > 0 {
						select {
						case sampleCh <- samples:
						case <-streamCtx.Done():
						}
					}
					logger.Debugf("[audio] 解码结束")
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
			totalBytes := int(frameCount) * int(sp.channels) * 2
			writePos := 0

			for writePos < totalBytes {
				if pos >= len(pcmData) {
					// 当前块播完，尝试获取下一块
					// 先用阻塞方式等待，避免不必要的静音间隙
					select {
					case chunk, ok := <-sampleCh:
						if !ok {
							// 所有数据播完，填充剩余部分为静音
							for i := writePos; i < totalBytes; i++ {
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
					}
				}

				end := pos + (totalBytes - writePos)
				if end > len(pcmData) {
					end = len(pcmData)
				}
				copied := copy(outputSamples[writePos:], pcmData[pos:end])
				pos = end
				writePos += copied
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

// downloadWithRetry 下载音频数据，支持网络中断后使用 Range 请求断点续传。
func (sp *StreamPlayer) downloadWithRetry(ctx context.Context, url string) ([]byte, error) {
	const maxRetries = 3
	var data []byte

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("创建请求失败: %w", err)
		}

		// 断点续传：从已下载的偏移量开始
		if len(data) > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", len(data)))
			logger.Debugf("[audio] 断点续传: 从 %d 字节处继续下载 (第 %d 次重试)", len(data), attempt)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attempt < maxRetries {
				logger.Debugf("[audio] 下载失败，%d 秒后重试: %v", attempt+1, err)
				select {
				case <-time.After(time.Duration(attempt+1) * time.Second):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				continue
			}
			return nil, fmt.Errorf("下载音频失败: %w", err)
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			if attempt < maxRetries {
				logger.Debugf("[audio] 下载返回状态码 %d，重试中", resp.StatusCode)
				select {
				case <-time.After(time.Duration(attempt+1) * time.Second):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				continue
			}
			return nil, fmt.Errorf("下载音频返回错误状态码: %d", resp.StatusCode)
		}

		// 读取所有数据
		chunk, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		data = append(data, chunk...)

		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if isNetworkError(err) && attempt < maxRetries {
				logger.Debugf("[audio] 读取中断(%d 字节已下载)，%d 秒后重试: %v", len(data), attempt+1, err)
				select {
				case <-time.After(time.Duration(attempt+1) * time.Second):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				continue
			}
			// 非网络错误或已耗尽重试次数，用已有数据继续
			if len(data) > 0 {
				logger.Debugf("[audio] 下载不完整(%d 字节)，使用已有数据: %v", len(data), err)
				return data, nil
			}
			return nil, fmt.Errorf("下载音频失败: %w", err)
		}

		// 下载成功
		logger.Debugf("[audio] 下载完成: %d 字节", len(data))
		return data, nil
	}

	// 重试耗尽，用已有数据
	if len(data) > 0 {
		logger.Debugf("[audio] 重试耗尽，使用已有数据: %d 字节", len(data))
		return data, nil
	}
	return nil, fmt.Errorf("下载音频失败: 重试耗尽")
}

// bytesReadSeeker 将 []byte 包装成 io.ReadSeeker（mp3.NewDecoder 需要）。
type bytesReadSeeker struct {
	data []byte
	pos  int
}

func newBytesReadSeeker(data []byte) *bytesReadSeeker {
	return &bytesReadSeeker{data: data}
}

func (b *bytesReadSeeker) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *bytesReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = int64(b.pos) + offset
	case io.SeekEnd:
		newPos = int64(len(b.data)) + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	if newPos < 0 {
		return 0, fmt.Errorf("negative position")
	}
	b.pos = int(newPos)
	return newPos, nil
}
