package pipeline

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/iabetor/pibuddy/internal/asr"
	"github.com/iabetor/pibuddy/internal/audio"
	"github.com/iabetor/pibuddy/internal/config"
	"github.com/iabetor/pibuddy/internal/llm"
	"github.com/iabetor/pibuddy/internal/tts"
	"github.com/iabetor/pibuddy/internal/vad"
	"github.com/iabetor/pibuddy/internal/wake"
)

// Pipeline 是主编排器，将所有组件串联在一起。
type Pipeline struct {
	cfg *config.Config

	capture *audio.Capture
	player  *audio.Player

	wakeDetector *wake.Detector
	vadDetector  *vad.Detector
	recognizer   *asr.Recognizer

	llmProvider    llm.Provider
	contextManager *llm.ContextManager

	ttsEngine tts.Engine

	state *StateMachine

	// cancelSpeak 在进入 Speaking 状态时设置；调用后可打断播放。
	cancelSpeak context.CancelFunc
	speakMu     sync.Mutex
}

// New 根据配置创建并初始化完整的 Pipeline。
func New(cfg *config.Config) (*Pipeline, error) {
	p := &Pipeline{
		cfg:   cfg,
		state: NewStateMachine(),
	}

	var err error

	// 音频采集（16kHz 单声道）
	p.capture, err = audio.NewCapture(cfg.Audio.SampleRate, cfg.Audio.Channels, cfg.Audio.FrameSize)
	if err != nil {
		return nil, fmt.Errorf("初始化音频采集失败: %w", err)
	}

	// 音频播放 —— TTS 通常输出 24kHz，默认使用此采样率
	p.player, err = audio.NewPlayer(24000, 1)
	if err != nil {
		p.capture.Close()
		return nil, fmt.Errorf("初始化音频播放失败: %w", err)
	}

	// 唤醒词检测器
	p.wakeDetector, err = wake.NewDetector(cfg.Wake.ModelPath, cfg.Wake.KeywordsFile, cfg.Wake.Threshold)
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("初始化唤醒词检测器失败: %w", err)
	}

	// 语音活动检测器
	p.vadDetector, err = vad.NewDetector(cfg.VAD.ModelPath, cfg.VAD.Threshold, cfg.VAD.MinSilenceMs)
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("初始化 VAD 失败: %w", err)
	}

	// 流式语音识别
	p.recognizer, err = asr.NewRecognizer(cfg.ASR.ModelPath, cfg.ASR.NumThreads)
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("初始化 ASR 失败: %w", err)
	}

	// 大模型提供者
	p.llmProvider = llm.NewOpenAIProvider(cfg.LLM.APIURL, cfg.LLM.APIKey, cfg.LLM.Model)
	p.contextManager = llm.NewContextManager(cfg.LLM.SystemPrompt, cfg.LLM.MaxHistory)

	// TTS 引擎
	switch cfg.TTS.Engine {
	case "tencent":
		p.ttsEngine, err = tts.NewTencentEngine(tts.TencentConfig{
			SecretID:  cfg.TTS.Tencent.SecretID,
			SecretKey: cfg.TTS.Tencent.SecretKey,
			VoiceType: cfg.TTS.Tencent.VoiceType,
			Region:    cfg.TTS.Tencent.Region,
		})
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("初始化腾讯云 TTS 失败: %w", err)
		}
	case "edge":
		p.ttsEngine = tts.NewEdgeEngine(cfg.TTS.Edge.Voice)
	case "piper":
		p.ttsEngine = tts.NewPiperEngine(cfg.TTS.Piper.ModelPath)
	default:
		p.Close()
		return nil, fmt.Errorf("未知的 TTS 引擎: %s", cfg.TTS.Engine)
	}

	log.Println("[pipeline] 所有组件初始化完成")
	return p, nil
}

// Run 启动主循环，阻塞直到 ctx 被取消。
func (p *Pipeline) Run(ctx context.Context) error {
	if err := p.capture.Start(); err != nil {
		return fmt.Errorf("启动音频采集失败: %w", err)
	}

	log.Println("[pipeline] 已启动 —— 请说唤醒词开始对话！")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case frame, ok := <-p.capture.C():
			if !ok {
				return nil
			}
			p.processFrame(ctx, frame)
		}
	}
}

// processFrame 根据当前状态将音频帧分发到对应的处理器。
func (p *Pipeline) processFrame(ctx context.Context, frame []float32) {
	switch p.state.Current() {
	case StateIdle:
		p.handleIdle(ctx, frame)
	case StateListening:
		p.handleListening(ctx, frame)
	case StateProcessing:
		// 处理中仍然监听唤醒词，允许打断慢速的 LLM 调用
		p.checkWakeInterrupt(frame)
	case StateSpeaking:
		// 播放中监听唤醒词，允许打断播放
		p.checkWakeInterrupt(frame)
	}
}

// handleIdle 在空闲状态下检测唤醒词。
func (p *Pipeline) handleIdle(_ context.Context, frame []float32) {
	if p.wakeDetector.Detect(frame) {
		log.Println("[pipeline] 检测到唤醒词！")
		p.vadDetector.Reset()
		p.recognizer.Reset()
		p.state.Transition(StateListening)
	}
}

// handleListening 同时将音频送入 VAD 和 ASR。
func (p *Pipeline) handleListening(ctx context.Context, frame []float32) {
	p.vadDetector.Feed(frame)
	p.recognizer.Feed(frame)

	// 获取实时部分识别结果（用于调试/反馈）
	text := p.recognizer.GetResult()
	if text != "" {
		log.Printf("[pipeline] 实时识别: %s", text)
	}

	// 检查用户是否停止说话
	if p.recognizer.IsEndpoint() {
		finalText := p.recognizer.GetResult()
		p.recognizer.Reset()
		p.vadDetector.Reset()

		if strings.TrimSpace(finalText) == "" {
			log.Println("[pipeline] ASR 结果为空，回到空闲状态")
			p.state.ForceIdle()
			return
		}

		log.Printf("[pipeline] ASR 最终结果: %s", finalText)
		p.state.Transition(StateProcessing)
		go p.processQuery(ctx, finalText)
	}
}

// checkWakeInterrupt 在非空闲状态下监听唤醒词以实现打断。
func (p *Pipeline) checkWakeInterrupt(frame []float32) {
	if p.wakeDetector.Detect(frame) {
		log.Println("[pipeline] 唤醒词打断！")
		p.interruptSpeak()
		p.vadDetector.Reset()
		p.recognizer.Reset()
		p.state.ForceIdle()
		p.state.Transition(StateListening)
	}
}

// processQuery 将识别文本发送给 LLM，流式接收回复，
// 按句拆分后逐句合成并播放。
func (p *Pipeline) processQuery(ctx context.Context, query string) {
	p.contextManager.Add("user", query)

	messages := p.contextManager.Messages()
	stream, err := p.llmProvider.ChatStream(ctx, messages)
	if err != nil {
		log.Printf("[pipeline] LLM 调用失败: %v", err)
		p.state.ForceIdle()
		return
	}

	// 累积完整回复，同时按句流式送 TTS
	var fullReply strings.Builder
	var sentenceBuf strings.Builder

	// 第一句准备好时切换到 Speaking 状态
	firstSentence := true

	for chunk := range stream {
		if p.state.Current() == StateIdle {
			// 已被打断
			return
		}

		fullReply.WriteString(chunk)
		sentenceBuf.WriteString(chunk)

		// 尝试提取完整句子进行流式 TTS
		for {
			sentence, rest, found := extractSentence(sentenceBuf.String())
			if !found {
				break
			}
			sentenceBuf.Reset()
			sentenceBuf.WriteString(rest)

			sentence = strings.TrimSpace(sentence)
			if sentence == "" {
				continue
			}

			if firstSentence {
				p.state.Transition(StateSpeaking)
				firstSentence = false
			}

			log.Printf("[pipeline] TTS 合成: %s", sentence)
			p.speakText(ctx, sentence)

			if p.state.Current() == StateIdle {
				return
			}
		}
	}

	// 刷新剩余文本
	remainder := strings.TrimSpace(sentenceBuf.String())
	if remainder != "" && p.state.Current() != StateIdle {
		if firstSentence {
			p.state.Transition(StateSpeaking)
		}
		log.Printf("[pipeline] TTS 合成剩余: %s", remainder)
		p.speakText(ctx, remainder)
	}

	p.contextManager.Add("assistant", fullReply.String())
	log.Printf("[pipeline] LLM 回复完成 (%d 字符)", fullReply.Len())

	p.state.ForceIdle()
}

// speakText 合成并播放单段文本。
func (p *Pipeline) speakText(ctx context.Context, text string) {
	samples, _, err := p.ttsEngine.Synthesize(ctx, text)
	if err != nil {
		log.Printf("[pipeline] TTS 合成失败: %v", err)
		return
	}

	speakCtx, cancel := context.WithCancel(ctx)
	p.speakMu.Lock()
	p.cancelSpeak = cancel
	p.speakMu.Unlock()

	defer func() {
		cancel()
		p.speakMu.Lock()
		p.cancelSpeak = nil
		p.speakMu.Unlock()
	}()

	if err := p.player.Play(speakCtx, samples); err != nil && err != context.Canceled {
		log.Printf("[pipeline] 播放失败: %v", err)
	}
}

// interruptSpeak 取消正在进行的语音播放。
func (p *Pipeline) interruptSpeak() {
	p.speakMu.Lock()
	if p.cancelSpeak != nil {
		p.cancelSpeak()
	}
	p.speakMu.Unlock()
}

// Close 释放所有资源。
func (p *Pipeline) Close() {
	log.Println("[pipeline] 正在关闭...")

	p.interruptSpeak()

	if p.capture != nil {
		p.capture.Close()
	}
	if p.player != nil {
		p.player.Close()
	}
	if p.wakeDetector != nil {
		p.wakeDetector.Close()
	}
	if p.vadDetector != nil {
		p.vadDetector.Close()
	}
	if p.recognizer != nil {
		p.recognizer.Close()
	}

	log.Println("[pipeline] 已关闭")
}

// extractSentence 尝试从文本中提取第一个完整句子。
// 返回 (sentence, remainder, found)。同时识别中文和英文的句末标点。
func extractSentence(text string) (string, string, bool) {
	sentenceEnders := []rune{'。', '！', '？', '；', '.', '!', '?', '\n'}
	for i, r := range text {
		for _, ender := range sentenceEnders {
			if r == ender {
				splitAt := i + utf8.RuneLen(r)
				return text[:splitAt], text[splitAt:], true
			}
		}
	}
	return "", text, false
}
