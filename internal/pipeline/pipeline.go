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
	"github.com/iabetor/pibuddy/internal/music"
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

	// 流式播放器（音乐）
	streamPlayer *audio.StreamPlayer

	// 连续对话超时
	continuousTimer  *time.Timer
	continuousMu     sync.Mutex
	lastActivityTime time.Time // 最后一次语音活动时间

	// 唤醒词防抖
	wakeCooldown   bool      // 是否处于冷却期
	wakeCooldownMu sync.Mutex
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

	// 初始化流式播放器（音乐）
	p.streamPlayer = audio.NewStreamPlayer(p.player)

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

	// 音乐工具
	if cfg.Tools.Music.Enabled {
		var musicProvider music.Provider
		musicProvider = music.NewNeteaseClient(cfg.Tools.Music.APIURL)

		// 创建播放历史存储
		musicHistory, err := music.NewHistoryStore(cfg.Tools.DataDir)
		if err != nil {
			log.Printf("[pipeline] 创建音乐历史存储失败: %v", err)
		}

		p.toolRegistry.Register(tools.NewMusicTool(tools.MusicConfig{
			Provider: musicProvider,
			History:  musicHistory,
			Enabled:  true,
		}))
		p.toolRegistry.Register(tools.NewListMusicHistoryTool(musicHistory))
	}

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
	case StateProcessing, StateSpeaking:
		// 播放/处理期间不处理音频帧，避免识别到播放内容
	}
}

// handleIdle 在空闲状态下检测唤醒词。
func (p *Pipeline) handleIdle(ctx context.Context, frame []float32) {
	// 检查是否在冷却期
	p.wakeCooldownMu.Lock()
	if p.wakeCooldown {
		p.wakeCooldownMu.Unlock()
		return
	}
	p.wakeCooldownMu.Unlock()

	if p.wakeDetector.Detect(frame) {
		log.Println("[pipeline] 检测到唤醒词！")

		// 进入冷却期，防止重复检测
		p.wakeCooldownMu.Lock()
		p.wakeCooldown = true
		p.wakeCooldownMu.Unlock()

		p.wakeDetector.Reset()
		p.vadDetector.Reset()
		p.recognizer.Reset()

		// 如果配置了唤醒回复语，先播放再进入监听
		if p.cfg.Dialog.WakeReply != "" {
			p.state.Transition(StateSpeaking)
			go p.playWakeReply(ctx)
		} else {
			p.state.Transition(StateListening)
			// 1秒后解除冷却期
			time.AfterFunc(1*time.Second, p.clearWakeCooldown)
		}
	}
}

// clearWakeCooldown 解除唤醒词冷却期。
func (p *Pipeline) clearWakeCooldown() {
	p.wakeCooldownMu.Lock()
	p.wakeCooldown = false
	p.wakeCooldownMu.Unlock()
}

// playWakeReply 播放唤醒回复语，完成后进入监听状态。
func (p *Pipeline) playWakeReply(ctx context.Context) {
	log.Printf("[pipeline] 播放唤醒回复: %s", p.cfg.Dialog.WakeReply)
	p.speakText(ctx, p.cfg.Dialog.WakeReply)

	// 播放完成后进入监听状态
	p.vadDetector.Reset()
	p.recognizer.Reset()
	p.state.SetState(StateListening)

	// 解除冷却期（延迟一点，确保不会立即重复检测）
	time.AfterFunc(500*time.Millisecond, p.clearWakeCooldown)
}

// handleListening 同时将音频送入 VAD 和 ASR。
func (p *Pipeline) handleListening(ctx context.Context, frame []float32) {
	p.vadDetector.Feed(frame)
	p.recognizer.Feed(frame)

	text := p.recognizer.GetResult()
	if text != "" {
		log.Printf("[pipeline] 实时识别: %s", text)
	}

	// 检测到语音活动，重置连续对话超时计时器
	if p.vadDetector.IsSpeech() {
		p.resetContinuousTimer()
	}

	if p.recognizer.IsEndpoint() {
		finalText := p.recognizer.GetResult()
		p.recognizer.Reset()
		p.vadDetector.Reset()

		// 停止连续对话计时器（用户正在说话，进入处理阶段）
		p.stopContinuousTimer()

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
			// 检查状态是否被重置（程序关闭时）
			currentState := p.state.Current()
			if currentState == StateIdle || currentState == StateListening {
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

				// 检查状态是否被重置
				currentState := p.state.Current()
				if currentState == StateIdle || currentState == StateListening {
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

		// 检查状态是否被重置
		if p.state.Current() == StateIdle || p.state.Current() == StateListening {
			return
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
			// 检查状态是否被重置
			if p.state.Current() == StateIdle || p.state.Current() == StateListening {
				return
			}

			log.Printf("[pipeline] 调用工具: %s(%s)", tc.Function.Name, tc.Function.Arguments)

			toolResult, err := p.toolRegistry.Execute(ctx, tc.Function.Name, json.RawMessage(tc.Function.Arguments))
			if err != nil {
				toolResult = fmt.Sprintf("工具执行失败: %v", err)
			}

			// 先添加工具结果到上下文（LLM 要求每个 tool_call 都有对应的 tool 消息）
			p.contextManager.AddMessage(llm.Message{
				Role:       "tool",
				Content:    toolResult,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			})

			// 检查是否是音乐播放结果
			if tc.Function.Name == "play_music" {
				var musicResult tools.MusicResult
				if jsonErr := json.Unmarshal([]byte(toolResult), &musicResult); jsonErr == nil {
					if musicResult.Success && musicResult.URL != "" {
						// 播放音乐
						log.Printf("[pipeline] 开始播放音乐: %s - %s", musicResult.Artist, musicResult.SongName)
						p.playMusic(ctx, musicResult.URL)
						// 音乐播放结束后继续
						return
					}
				}
			}
		}
		// 继续下一轮 LLM 调用
	}

	// 回复完成后进入连续对话模式（等待用户继续说）
	// 但如果已经被打断（状态是 Listening），则不进入
	curState := p.state.Current()
	if curState != StateIdle && curState != StateListening {
		p.enterContinuousMode()
	}
}

// enterContinuousMode 进入连续对话模式。
// 回复完成后不立即回到空闲，而是进入监听状态并启动超时计时器。
func (p *Pipeline) enterContinuousMode() {
	if p.cfg.Dialog.ContinuousTimeout <= 0 {
		// 连续对话模式禁用，直接回到空闲
		p.state.ForceIdle()
		return
	}

	// 进入监听状态
	p.vadDetector.Reset()
	p.recognizer.Reset()
	p.state.ForceIdle() // 先重置
	p.state.Transition(StateListening)

	// 启动超时计时器
	p.startContinuousTimer()
	log.Printf("[pipeline] 进入连续对话模式，%d 秒内无输入将回到空闲", p.cfg.Dialog.ContinuousTimeout)
}

// startContinuousTimer 启动连续对话超时计时器。
func (p *Pipeline) startContinuousTimer() {
	p.continuousMu.Lock()
	defer p.continuousMu.Unlock()

	// 停止之前的计时器
	if p.continuousTimer != nil {
		p.continuousTimer.Stop()
	}

	// 记录当前时间作为最后活动时间
	p.lastActivityTime = time.Now()

	// 启动新计时器
	p.continuousTimer = time.AfterFunc(time.Duration(p.cfg.Dialog.ContinuousTimeout)*time.Second, func() {
		p.continuousMu.Lock()
		// 检查是否超时（如果期间有语音活动，不触发）
		if time.Since(p.lastActivityTime) >= time.Duration(p.cfg.Dialog.ContinuousTimeout)*time.Second {
			p.continuousMu.Unlock()
			// 超时，回到空闲状态
			if p.state.Current() == StateListening {
				log.Println("[pipeline] 连续对话超时，回到空闲状态")
				p.state.ForceIdle()
			}
		} else {
			p.continuousMu.Unlock()
		}
	})
}

// stopContinuousTimer 停止连续对话超时计时器。
func (p *Pipeline) stopContinuousTimer() {
	p.continuousMu.Lock()
	defer p.continuousMu.Unlock()

	if p.continuousTimer != nil {
		p.continuousTimer.Stop()
		p.continuousTimer = nil
	}
}

// resetContinuousTimer 重置连续对话超时计时器（检测到语音活动时调用）。
func (p *Pipeline) resetContinuousTimer() {
	p.continuousMu.Lock()
	p.lastActivityTime = time.Now()
	p.continuousMu.Unlock()
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
		return
	}

	// 播放完成后，如果当前状态是 Speaking，回到 Processing
	// 这样如果有后续内容（如工具调用结果），可以再次转换到 Speaking
	if p.state.Current() == StateSpeaking {
		p.state.SetState(StateProcessing)
	}
}

// interruptSpeak 取消正在进行的语音播放。
func (p *Pipeline) interruptSpeak() {
	p.speakMu.Lock()
	if p.cancelSpeak != nil {
		p.cancelSpeak()
	}
	p.speakMu.Unlock()

	// 也停止音乐播放
	if p.streamPlayer != nil {
		p.streamPlayer.Stop()
	}
}

// playMusic 播放音乐。
func (p *Pipeline) playMusic(ctx context.Context, url string) {
	// 确保状态为 Speaking
	if p.state.Current() != StateSpeaking {
		p.state.SetState(StateSpeaking)
	}

	if err := p.streamPlayer.Play(ctx, url); err != nil {
		if err != context.Canceled {
			log.Printf("[pipeline] 音乐播放失败: %v", err)
		}
	}

	// 播放完成后进入连续对话模式
	p.enterContinuousMode()
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
