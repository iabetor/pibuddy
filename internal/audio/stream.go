package audio

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/hajimehoshi/go-mp3"
)

// StreamPlayer 支持从 HTTP URL 流式播放 MP3 音频。
type StreamPlayer struct {
	player *Player
	mu     sync.Mutex
	cancel context.CancelFunc
}

// NewStreamPlayer 创建流式播放器。
func NewStreamPlayer(player *Player) *StreamPlayer {
	return &StreamPlayer{
		player: player,
	}
}

// Play 从 URL 流式下载并播放 MP3 音频。
// 使用生产者-消费者模式，后台预加载，前台播放。
func (sp *StreamPlayer) Play(ctx context.Context, url string) error {
	sp.mu.Lock()
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
	log.Printf("[audio] 流式播放: 采样率 %d Hz", sampleRate)

	// 创建缓冲区通道，预缓冲 5 个块（每块约 1 秒）
	const chunkSize = 44100 // 约 1 秒的样本数
	const bufferChunks = 5
	const preBufferChunks = 3 // 开始播放前至少缓冲的块数
	chunkCh := make(chan []float32, bufferChunks)
	errCh := make(chan error, 1)

	// 生产者：后台解码
	go func() {
		defer close(chunkCh)

		buf := make([]byte, 8192) // 更大的读取缓冲区
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
					// 发送剩余数据
					if len(samples) > 0 {
						select {
						case chunkCh <- samples:
						case <-streamCtx.Done():
						}
					}
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

			// int16 立体声转单声道 float32
			chunkSamples := int16StereoToMonoFloat32(buf[:n])
			samples = append(samples, chunkSamples...)

			// 累积到一定量后发送
			for len(samples) >= chunkSize {
				chunk := make([]float32, chunkSize)
				copy(chunk, samples[:chunkSize])
				samples = samples[chunkSize:]

				select {
				case chunkCh <- chunk:
				case <-streamCtx.Done():
					return
				}
			}
		}
	}()

	// 预缓冲：等待缓冲区填充到一定程度
	preBuffer := make([][]float32, 0, preBufferChunks)
preBufferLoop:
	for len(preBuffer) < preBufferChunks {
		select {
		case <-streamCtx.Done():
			log.Println("[audio] 预缓冲被取消")
			return streamCtx.Err()
		case err := <-errCh:
			return err
		case chunk, ok := <-chunkCh:
			if !ok {
				// 文件较短，直接播放已有的
				break preBufferLoop
			}
			preBuffer = append(preBuffer, chunk)
			log.Printf("[audio] 预缓冲 %d/%d", len(preBuffer), preBufferChunks)
		}
	}
	log.Printf("[audio] 预缓冲完成，开始播放")

	// 消费者：前台播放（先播放预缓冲的数据）
	for _, chunk := range preBuffer {
		if err := sp.player.Play(streamCtx, chunk, sampleRate); err != nil {
			if err == context.Canceled {
				return err
			}
			return fmt.Errorf("播放失败: %w", err)
		}
	}

	// 继续播放后续数据
	for {
		select {
		case <-streamCtx.Done():
			log.Println("[audio] 流式播放被取消")
			return streamCtx.Err()
		case err := <-errCh:
			return err
		case chunk, ok := <-chunkCh:
			if !ok {
				// 通道关闭，播放完成
				log.Println("[audio] 流式播放完成")
				return nil
			}
			if err := sp.player.Play(streamCtx, chunk, sampleRate); err != nil {
				if err == context.Canceled {
					return err
				}
				return fmt.Errorf("播放失败: %w", err)
			}
		}
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

// int16StereoToMonoFloat32 将 int16 立体声 PCM 转换为单声道 float32。
func int16StereoToMonoFloat32(data []byte) []float32 {
	numSamples := len(data) / 4
	if numSamples == 0 {
		return nil
	}
	samples := make([]float32, numSamples)

	for i := 0; i < numSamples; i++ {
		// 小端序：低字节在前
		left := int16(data[i*4]) | int16(data[i*4+1])<<8
		right := int16(data[i*4+2]) | int16(data[i*4+3])<<8
		samples[i] = (float32(left) + float32(right)) / 65536.0
	}

	return samples
}
