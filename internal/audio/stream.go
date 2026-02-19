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
// 使用边下载边播放的流式架构，减少首次播放延迟。
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

	// 创建流式缓冲，边下载边解码
	sb := newStreamingBuffer()

	// 后台下载 goroutine：将数据流式写入 streamingBuffer
	go sp.streamDownload(streamCtx, url, sb)

	// 等待至少 32KB 数据到达再初始化解码器（MP3 帧头 + 几帧数据）
	waitStart := time.Now()
	for sb.Len() < 32768 {
		select {
		case <-streamCtx.Done():
			return streamCtx.Err()
		case <-time.After(10 * time.Millisecond):
		}
		// 如果下载已完成（可能文件很小），也跳出
		sb.mu.Lock()
		done := sb.finished
		sb.mu.Unlock()
		if done {
			break
		}
	}
	if sb.Len() == 0 {
		return nil
	}
	logger.Debugf("[audio] 等待首批数据: %d 字节, 耗时 %v", sb.Len(), time.Since(waitStart).Round(time.Millisecond))

	// 解码 MP3（streamingBuffer 实现了 io.ReadSeeker）
	decoder, err := mp3.NewDecoder(sb)
	if err != nil {
		return fmt.Errorf("创建 MP3 解码器失败: %w", err)
	}

	sampleRate := decoder.SampleRate()
	logger.Debugf("[audio] 流式播放: 采样率 %d Hz", sampleRate)

	// 创建音频数据通道
	chunkSize := sampleRate * 2 // 约 2 秒的样本数
	const bufferChunks = 5
	sampleCh := make(chan []float32, bufferChunks)
	errCh := make(chan error, 1)

	// 生产者：后台解码（从 streamingBuffer 读取，会自动等待下载数据）
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

	// 预缓冲：只等 1 块数据即可开始播放（降低延迟）
	preBuffer := make([][]float32, 0, 1)
preBufferLoop:
	for len(preBuffer) < 1 {
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
			logger.Debugf("[audio] 预缓冲 %d/1", len(preBuffer))
		}
	}
	if len(preBuffer) == 0 {
		return nil // 空文件
	}
	logger.Debugf("[audio] 预缓冲完成，开始播放 (总延迟 %v)", time.Since(waitStart).Round(time.Millisecond))

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

// streamDownload 流式下载音频数据到 streamingBuffer，支持网络中断后断点续传。
func (sp *StreamPlayer) streamDownload(ctx context.Context, url string, sb *streamingBuffer) {
	const maxRetries = 3

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			sb.Finish(fmt.Errorf("创建请求失败: %w", err))
			return
		}

		// 断点续传：从已下载的偏移量开始
		downloaded := sb.Len()
		if downloaded > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", downloaded))
			logger.Debugf("[audio] 断点续传: 从 %d 字节处继续下载 (第 %d 次重试)", downloaded, attempt)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				sb.Finish(ctx.Err())
				return
			}
			if attempt < maxRetries {
				logger.Debugf("[audio] 下载失败，%d 秒后重试: %v", attempt+1, err)
				select {
				case <-time.After(time.Duration(attempt+1) * time.Second):
				case <-ctx.Done():
					sb.Finish(ctx.Err())
					return
				}
				continue
			}
			sb.Finish(fmt.Errorf("下载音频失败: %w", err))
			return
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			if attempt < maxRetries {
				logger.Debugf("[audio] 下载返回状态码 %d，重试中", resp.StatusCode)
				select {
				case <-time.After(time.Duration(attempt+1) * time.Second):
				case <-ctx.Done():
					sb.Finish(ctx.Err())
					return
				}
				continue
			}
			sb.Finish(fmt.Errorf("下载音频返回错误状态码: %d", resp.StatusCode))
			return
		}

		// 流式读取，每读一块就追加到 streamingBuffer
		buf := make([]byte, 32768) // 32KB chunks
		readErr := func() error {
			defer resp.Body.Close()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				n, err := resp.Body.Read(buf)
				if n > 0 {
					chunk := make([]byte, n)
					copy(chunk, buf[:n])
					sb.Append(chunk)
				}
				if err != nil {
					if err == io.EOF {
						return nil // 下载完成
					}
					return err
				}
			}
		}()

		if readErr == nil {
			// 下载成功完成
			logger.Debugf("[audio] 下载完成: %d 字节", sb.Len())
			sb.Finish(nil)
			return
		}

		if ctx.Err() != nil {
			sb.Finish(ctx.Err())
			return
		}

		if isNetworkError(readErr) && attempt < maxRetries {
			logger.Debugf("[audio] 读取中断(%d 字节已下载)，%d 秒后重试: %v", sb.Len(), attempt+1, readErr)
			select {
			case <-time.After(time.Duration(attempt+1) * time.Second):
			case <-ctx.Done():
				sb.Finish(ctx.Err())
				return
			}
			continue
		}

		// 非网络错误或重试耗尽
		if sb.Len() > 0 {
			logger.Debugf("[audio] 下载不完整(%d 字节)，使用已有数据: %v", sb.Len(), readErr)
			sb.Finish(nil) // 已有数据可用
			return
		}
		sb.Finish(fmt.Errorf("下载音频失败: %w", readErr))
		return
	}

	// 重试耗尽
	if sb.Len() > 0 {
		logger.Debugf("[audio] 重试耗尽，使用已有数据: %d 字节", sb.Len())
		sb.Finish(nil)
	} else {
		sb.Finish(fmt.Errorf("下载音频失败: 重试耗尽"))
	}
}

// streamingBuffer 是一个边下载边可读的 io.ReadSeeker 实现。
// HTTP 下载 goroutine 通过 Append 写入数据，Finish 标记下载完成。
// go-mp3 解码器通过 Read/Seek 接口消费数据。
// 当 Read 到达缓冲末尾但下载未完成时，会阻塞等待更多数据。
type streamingBuffer struct {
	mu       sync.Mutex
	cond     *sync.Cond
	data     []byte
	pos      int
	finished bool // 下载完成标记
	err      error // 下载出错
}

func newStreamingBuffer() *streamingBuffer {
	sb := &streamingBuffer{}
	sb.cond = sync.NewCond(&sb.mu)
	return sb
}

// Append 由下载 goroutine 调用，追加数据到缓冲。
func (sb *streamingBuffer) Append(chunk []byte) {
	sb.mu.Lock()
	sb.data = append(sb.data, chunk...)
	sb.mu.Unlock()
	sb.cond.Broadcast()
}

// Finish 标记下载完成（正常或出错）。
func (sb *streamingBuffer) Finish(err error) {
	sb.mu.Lock()
	sb.finished = true
	sb.err = err
	sb.mu.Unlock()
	sb.cond.Broadcast()
}

// Len 返回当前已缓冲的数据长度。
func (sb *streamingBuffer) Len() int {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return len(sb.data)
}

// Read 实现 io.Reader。读到缓冲末尾时，如果下载未完成则阻塞等待。
func (sb *streamingBuffer) Read(p []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	for {
		// 有数据可读
		if sb.pos < len(sb.data) {
			n := copy(p, sb.data[sb.pos:])
			sb.pos += n
			return n, nil
		}

		// 没有数据了
		if sb.finished {
			if sb.err != nil {
				return 0, sb.err
			}
			return 0, io.EOF
		}

		// 等待更多数据
		sb.cond.Wait()
	}
}

// Seek 实现 io.Seeker。支持 go-mp3 解码器初始化时的 seek 操作。
func (sb *streamingBuffer) Seek(offset int64, whence int) (int64, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = int64(sb.pos) + offset
	case io.SeekEnd:
		// go-mp3 在初始化时用 SeekEnd 探测文件长度。
		// 如果下载还没完成，需要等待足够长或者返回当前已有长度。
		// 实际上 go-mp3 初始化只需要读几帧就能确定采样率，
		// 所以返回当前长度即可。
		if !sb.finished {
			// 等到有足够数据（至少 16KB，足够 MP3 初始化）
			for len(sb.data) < 16384 && !sb.finished {
				sb.cond.Wait()
			}
		}
		newPos = int64(len(sb.data)) + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	if newPos < 0 {
		return 0, fmt.Errorf("negative position")
	}
	sb.pos = int(newPos)
	return newPos, nil
}
