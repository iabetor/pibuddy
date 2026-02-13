package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/iabetor/pibuddy/internal/asr"
	"github.com/iabetor/pibuddy/internal/audio"
	"github.com/iabetor/pibuddy/internal/config"
	"github.com/iabetor/pibuddy/internal/llm"
	"github.com/iabetor/pibuddy/internal/tools"
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

	toolRegistry *tools.Registry
	alarmStore   *tools.AlarmStore

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

	// 音频播放（单声道，采样率由 TTS 动态返回）
	p.player, err = audio.NewPlayer(1)
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
			Speed:     cfg.TTS.Tencent.Speed,
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

	// 初始化工具
	if err := p.initTools(cfg); err != nil {
		p.Close()
		return nil, fmt.Errorf("初始化工具失败: %w", err)
	}

	log.Println("[pipeline] 所有组件初始化完成")
	return p, nil
}

// initTools 注册所有可用工具。
func (p *Pipeline) initTools(cfg *config.Config) error {
	p.toolRegistry = tools.NewRegistry()

	// 本地工具
	p.toolRegistry.Register(tools.NewDateTimeTool())
	p.toolRegistry.Register(tools.NewCalculatorTool())

	// 天气工具
	if cfg.Tools.Weather.CredentialID != "" || cfg.Tools.Weather.APIKey != "" {
		p.toolRegistry.Register(tools.NewWeatherTool(tools.WeatherConfig{
			APIKey:         cfg.Tools.Weather.APIKey,
			APIHost:        cfg.Tools.Weather.APIHost,
			CredentialID:   cfg.Tools.Weather.CredentialID,
			ProjectID:      cfg.Tools.Weather.ProjectID,
			PrivateKeyPath: cfg.Tools.Weather.PrivateKeyPath,
		}))
	}

	// 闹钟工具
	var err error
	p.alarmStore, err = tools.NewAlarmStore(cfg.Tools.DataDir)
	if err != nil {
		return fmt.Errorf("初始化闹钟存储失败: %w", err)
	}
	p.toolRegistry.Register(tools.NewSetAlarmTool(p.alarmStore))
	p.toolRegistry.Register(tools.NewListAlarmsTool(p.alarmStore))
	p.toolRegistry.Register(tools.NewDeleteAlarmTool(p.alarmStore))

	// 备忘录工具
	memoStore, err := tools.NewMemoStore(cfg.Tools.DataDir)
	if err != nil {
		return fmt.Errorf("初始化备忘录存储失败: %w", err)
	}
	p.toolRegistry.Register(tools.NewAddMemoTool(memoStore))
	p.toolRegistry.Register(tools.NewListMemosTool(memoStore))
	p.toolRegistry.Register(tools.NewDeleteMemoTool(memoStore))

	// 新闻和股票
	p.toolRegistry.Register(tools.NewNewsTool())
	p.toolRegistry.Register(tools.NewStockTool())

	log.Printf("[pipeline] 已注册 %d 个工具", p.toolRegistry.Count())
	return nil
}

// Run 启动主循环，阻塞直到 ctx 被取消。
func (p *Pipeline) Run(ctx context.Context) error {
	if err := p.capture.Start(); err != nil {
		return fmt.Errorf("启动音频采集失败: %w", err)
	}

	// 启动闹钟检查 goroutine
	go p.alarmChecker(ctx)

	log.Println("[pipeline] 已启动 — 请说唤醒词开始对话！")

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

// alarmChecker 每 30 秒检查一次到期闹钟，到期时 TTS 播报。
func (p *Pipeline) alarmChecker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dueAlarms := p.alarmStore.PopDueAlarms()
			for _, a := range dueAlarms {
				log.Printf("[pipeline] 闹钟到期: %s", a.Message)
				msg := fmt.Sprintf("闹钟提醒: %s", a.Message)
				p.speakText(ctx, msg)
			}
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
		p.checkWakeInterrupt(frame)
	case StateSpeaking:
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

	text := p.recognizer.GetResult()
	if text != "" {
		log.Printf("[pipeline] 实时识别: %s", text)
	}

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

// processQuery 将识别文本发送给 LLM，支持工具调用循环。
// 流式接收回复，按句拆分后逐句合成并播放。
func (p *Pipeline) processQuery(ctx context.Context, query string) {
	p.contextManager.Add("user", query)

	toolDefs := p.toolRegistry.Definitions()
	maxRounds := 3

	for round := 0; round < maxRounds; round++ {
		messages := p.contextManager.Messages()

		textCh, resultCh, err := p.llmProvider.ChatStreamWithTools(ctx, messages, toolDefs)
		if err != nil {
			log.Printf("[pipeline] LLM 调用失败: %v", err)
			p.state.ForceIdle()
			return
		}

		// 流式消费文本并 TTS
		var fullReply strings.Builder
		var sentenceBuf strings.Builder
		firstSentence := true

		for chunk := range textCh {
			if p.state.Current() == StateIdle {
				// 等待 resultCh 排空
				for range resultCh {
				}
				return
			}

			fullReply.WriteString(chunk)
			sentenceBuf.WriteString(chunk)

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
					for range resultCh {
					}
					return
				}
			}
		}

		// 获取最终结果（包含可能的 tool_calls）
		result := <-resultCh
		if result == nil {
			break
		}

		// 如果没有工具调用，处理剩余文本并结束
		if len(result.ToolCalls) == 0 {
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
			break
		}

		// 有工具调用 — 执行工具，然后继续下一轮
		log.Printf("[pipeline] 第 %d 轮工具调用: %d 个工具", round+1, len(result.ToolCalls))

		// 将 assistant 消息（含 tool_calls）添加到上下文
		assistantMsg := llm.Message{
			Role:      "assistant",
			Content:   result.Content,
			ToolCalls: result.ToolCalls,
		}
		p.contextManager.AddMessage(assistantMsg)

		// 执行每个工具并将结果添加到上下文
		for _, tc := range result.ToolCalls {
			log.Printf("[pipeline] 调用工具: %s(%s)", tc.Function.Name, tc.Function.Arguments)

			toolResult, err := p.toolRegistry.Execute(ctx, tc.Function.Name, json.RawMessage(tc.Function.Arguments))
			if err != nil {
				toolResult = fmt.Sprintf("工具执行失败: %v", err)
			}

			p.contextManager.AddMessage(llm.Message{
				Role:       "tool",
				Content:    toolResult,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			})
		}
		// 继续下一轮 LLM 调用
	}

	p.state.ForceIdle()
}

// speakText 合成并播放单段文本。
func (p *Pipeline) speakText(ctx context.Context, text string) {
	samples, sampleRate, err := p.ttsEngine.Synthesize(ctx, text)
	if err != nil {
		log.Printf("[pipeline] TTS 合成失败: %v", err)
		return
	}
	if len(samples) == 0 {
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

	if err := p.player.Play(speakCtx, samples, sampleRate); err != nil && err != context.Canceled {
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
