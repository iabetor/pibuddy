package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/iabetor/pibuddy/internal/asr"
	"github.com/iabetor/pibuddy/internal/audio"
	"github.com/iabetor/pibuddy/internal/config"
	"github.com/iabetor/pibuddy/internal/llm"
	"github.com/iabetor/pibuddy/internal/logger"
	"github.com/iabetor/pibuddy/internal/music"
	"github.com/iabetor/pibuddy/internal/rss"
	"github.com/iabetor/pibuddy/internal/tools"
	"github.com/iabetor/pibuddy/internal/tts"
	"github.com/iabetor/pibuddy/internal/vad"
	"github.com/iabetor/pibuddy/internal/voiceprint"
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

	ttsEngine        tts.Engine
	fallbackTtsEngine tts.Engine // 回退 TTS 引擎（网络失败时使用）

	toolRegistry *tools.Registry
	alarmStore   *tools.AlarmStore
	timerStore   *tools.TimerStore
	volumeCtrl   tools.VolumeController

	state *StateMachine

	// cancelSpeak 在进入 Speaking 状态时设置；调用后可打断播放。
	cancelSpeak context.CancelFunc
	speakMu     sync.Mutex

	// cancelQuery 在进入 Processing 状态时设置；打断时取消 LLM 调用。
	cancelQuery context.CancelFunc
	queryMu     sync.Mutex

	// 流式播放器（音乐）
	streamPlayer *audio.StreamPlayer

	// 音乐缓存
	musicCache *audio.MusicCache

	// 音乐播放列表
	playlist *music.Playlist

	// 连续对话超时
	continuousTimer *time.Timer
	continuousMu    sync.Mutex

	// 唤醒词防抖
	wakeCooldown   bool      // 是否处于冷却期
	wakeCooldownMu sync.Mutex

	// 打断标志（跨 goroutine 通信，通知 processQuery 退出）
	interrupted atomic.Bool

	// 声纹识别
	voiceprintMgr     *voiceprint.Manager
	voiceprintBuf     []float32
	voiceprintBufMu   sync.Mutex
	voiceprintBufSize int // 目标缓冲大小 = BufferSecs * SampleRate
	voiceprintWg      sync.WaitGroup // 等待声纹识别完成
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
	p.recognizer, err = asr.NewRecognizer(
		cfg.ASR.ModelPath,
		cfg.ASR.NumThreads,
		cfg.ASR.Rule1MinTrailingSilence,
		cfg.ASR.Rule2MinTrailingSilence,
		cfg.ASR.Rule3MinUtteranceLength,
	)
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
	case "say":
		p.ttsEngine = tts.NewSayEngine(cfg.TTS.Say.Voice)
	default:
		p.Close()
		return nil, fmt.Errorf("未知的 TTS 引擎: %s", cfg.TTS.Engine)
	}

	// 初始化备用 TTS 引擎（网络失败时使用）
	if cfg.TTS.Fallback != "" && cfg.TTS.Fallback != cfg.TTS.Engine {
		switch cfg.TTS.Fallback {
		case "piper":
			if cfg.TTS.Piper.ModelPath != "" {
				p.fallbackTtsEngine = tts.NewPiperEngine(cfg.TTS.Piper.ModelPath)
				logger.Info("[pipeline] 已启用 TTS 回退引擎: piper")
			}
		case "edge":
			p.fallbackTtsEngine = tts.NewEdgeEngine(cfg.TTS.Edge.Voice)
			logger.Info("[pipeline] 已启用 TTS 回退引擎: edge")
		case "say":
			p.fallbackTtsEngine = tts.NewSayEngine(cfg.TTS.Say.Voice)
			logger.Info("[pipeline] 已启用 TTS 回退引擎: say (macOS)")
		default:
			logger.Warnf("[pipeline] 未知的 TTS 回退引擎: %s", cfg.TTS.Fallback)
		}
	}

	// 初始化工具
	if err := p.initTools(cfg); err != nil {
		p.Close()
		return nil, fmt.Errorf("初始化工具失败: %w", err)
	}

	// 初始化流式播放器（音乐）
	streamPlayer, err := audio.NewStreamPlayer(1) // 单声道
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("初始化流式播放器失败: %w", err)
	}
	p.streamPlayer = streamPlayer

	// 初始化声纹识别（可选，失败不阻止启动）
	if cfg.Voiceprint.Enabled && cfg.Voiceprint.ModelPath != "" {
		vpMgr, vpErr := voiceprint.NewManager(cfg.Voiceprint, cfg.Tools.DataDir)
		if vpErr != nil {
			logger.Warnf("[pipeline] 声纹识别初始化失败（已禁用）: %v", vpErr)
		} else {
			p.voiceprintMgr = vpMgr
			p.voiceprintBufSize = int(cfg.Voiceprint.BufferSecs * float32(cfg.Audio.SampleRate))

			// 设置主人（如果配置了主人姓名）
			if cfg.Voiceprint.OwnerName != "" {
				if err := vpMgr.SetOwner(cfg.Voiceprint.OwnerName); err != nil {
					logger.Warnf("[pipeline] 设置主人失败: %v", err)
				} else {
					logger.Infof("[pipeline] 已设置主人: %s", cfg.Voiceprint.OwnerName)
				}
			}
		}
	}

	logger.Info("[pipeline] 所有组件初始化完成")
	return p, nil
}

// initTools 注册所有可用工具。
func (p *Pipeline) initTools(cfg *config.Config) error {
	p.toolRegistry = tools.NewRegistry()

	// 本地工具
	p.toolRegistry.Register(tools.NewDateTimeTool())
	p.toolRegistry.Register(tools.NewCalculatorTool())
	p.toolRegistry.Register(tools.NewLunarDateTool())

	// 天气工具
	if cfg.Tools.Weather.CredentialID != "" || cfg.Tools.Weather.APIKey != "" {
		weatherTool := tools.NewWeatherTool(tools.WeatherConfig{
			APIKey:         cfg.Tools.Weather.APIKey,
			APIHost:        cfg.Tools.Weather.APIHost,
			CredentialID:   cfg.Tools.Weather.CredentialID,
			ProjectID:      cfg.Tools.Weather.ProjectID,
			PrivateKeyPath: cfg.Tools.Weather.PrivateKeyPath,
		})
		p.toolRegistry.Register(weatherTool)
		// 空气质量工具（复用天气工具的认证）
		p.toolRegistry.Register(tools.NewAirQualityTool(weatherTool))
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

		// 根据 provider 配置选择音乐平台
		switch cfg.Tools.Music.Provider {
		case "qq":
			apiURL := cfg.Tools.Music.QQ.APIURL
			if apiURL == "" {
				apiURL = "http://localhost:3300"
			}
			musicProvider = music.NewQQMusicClientWithDataDir(apiURL, cfg.Tools.DataDir)
			logger.Infof("[pipeline] 使用 QQ 音乐 (API: %s)", apiURL)
		default:
			// 默认使用网易云音乐
			apiURL := cfg.Tools.Music.Netease.APIURL
			if apiURL == "" {
				apiURL = cfg.Tools.Music.APIURL // 兼容旧配置
			}
			if apiURL == "" {
				apiURL = "http://localhost:3000"
			}
			musicProvider = music.NewNeteaseClientWithDataDir(apiURL, cfg.Tools.DataDir)
			logger.Infof("[pipeline] 使用网易云音乐 (API: %s)", apiURL)
		}

		// 创建播放历史存储
		musicHistory, err := music.NewHistoryStore(cfg.Tools.DataDir)
		if err != nil {
			logger.Warnf("[pipeline] 创建音乐历史存储失败: %v", err)
		}

		// 创建音乐缓存
		var musicCache *audio.MusicCache
		musicCache, err = audio.NewMusicCache(cfg.Tools.Music.CacheDir, cfg.Tools.Music.CacheMaxSize)
		if err != nil {
			logger.Warnf("[pipeline] 创建音乐缓存失败（缓存已禁用）: %v", err)
		} else if musicCache.Enabled() {
			logger.Infof("[pipeline] 音乐缓存已启用: %s (上限 %dMB)", cfg.Tools.Music.CacheDir, cfg.Tools.Music.CacheMaxSize)
		}
		p.musicCache = musicCache

		// 创建播放列表
		p.playlist = music.NewPlaylist(musicProvider, musicHistory)

		musicCfg := tools.MusicConfig{
			Provider: musicProvider,
			History:  musicHistory,
			Playlist: p.playlist,
			Cache:    musicCache,
			Enabled:  true,
		}
		p.toolRegistry.Register(tools.NewSearchMusicTool(musicCfg))
		p.toolRegistry.Register(tools.NewPlayMusicTool(musicCfg))
		p.toolRegistry.Register(tools.NewListMusicHistoryTool(musicHistory))
		p.toolRegistry.Register(tools.NewNextMusicTool(p.playlist))
		p.toolRegistry.Register(tools.NewSetPlayModeTool(p.playlist))
		if musicCache != nil && musicCache.Enabled() {
			p.toolRegistry.Register(tools.NewListMusicCacheTool(musicCache))
			p.toolRegistry.Register(tools.NewDeleteMusicCacheTool(musicCache))
		}
	}

	// RSS 订阅工具
	if cfg.Tools.RSS.Enabled {
		feedStore, err := rss.NewFeedStore(cfg.Tools.DataDir)
		if err != nil {
			logger.Warnf("[pipeline] 初始化 RSS 存储失败: %v", err)
		} else {
			fetcher := rss.NewFetcher(feedStore, cfg.Tools.DataDir, cfg.Tools.RSS.CacheTTL)
			p.toolRegistry.Register(tools.NewAddRSSFeedTool(feedStore, fetcher))
			p.toolRegistry.Register(tools.NewListRSSFeedsTool(feedStore))
			p.toolRegistry.Register(tools.NewDeleteRSSFeedTool(feedStore))
			p.toolRegistry.Register(tools.NewGetRSSNewsTool(feedStore, fetcher))
			logger.Infof("[pipeline] RSS 订阅功能已启用")
		}
	}

	// 声纹管理工具（仅主人可用）
	if p.voiceprintMgr != nil {
		vpCfg := tools.VoiceprintConfig{
			Manager:    p.voiceprintMgr,
			Capture:    p.capture,
			SampleRate: cfg.Audio.SampleRate,
			OwnerName:  cfg.Voiceprint.OwnerName,
		}
		p.toolRegistry.Register(tools.NewRegisterVoiceprintTool(vpCfg))
		p.toolRegistry.Register(tools.NewDeleteVoiceprintTool(vpCfg))
		p.toolRegistry.Register(tools.NewSetPreferencesTool(vpCfg))
	}

	// 倒计时工具
	p.timerStore, err = tools.NewTimerStore(cfg.Tools.DataDir, func(entry tools.TimerEntry) {
		// 倒计时到期回调
		logger.Infof("[pipeline] 倒计时到期: %s", entry.ID)
		var msg string
		if entry.Label != "" {
			msg = fmt.Sprintf("%s提醒时间到了", entry.Label)
		} else {
			msg = "倒计时结束了"
		}
		p.speakText(context.Background(), msg)
	})
	if err != nil {
		return fmt.Errorf("初始化倒计时存储失败: %w", err)
	}
	p.toolRegistry.Register(tools.NewSetTimerTool(p.timerStore))
	p.toolRegistry.Register(tools.NewListTimersTool(p.timerStore))
	p.toolRegistry.Register(tools.NewCancelTimerTool(p.timerStore))

	// 音量控制工具
	p.volumeCtrl, err = tools.NewVolumeController()
	if err != nil {
		logger.Warnf("[pipeline] 音量控制器初始化失败（已禁用）: %v", err)
	} else {
		p.toolRegistry.Register(tools.NewSetVolumeTool(p.volumeCtrl, tools.VolumeConfig{
			Step: cfg.Tools.Volume.Step,
		}))
		p.toolRegistry.Register(tools.NewGetVolumeTool(p.volumeCtrl))
	}

	// 翻译工具
	if cfg.Tools.Translate.Enabled && cfg.Tools.Translate.SecretID != "" {
		translateTool, err := tools.NewTranslateTool(
			cfg.Tools.Translate.SecretID,
			cfg.Tools.Translate.SecretKey,
			cfg.Tools.Translate.Region,
		)
		if err != nil {
			logger.Warnf("[pipeline] 翻译工具初始化失败: %v", err)
		} else {
			p.toolRegistry.Register(translateTool)
			logger.Info("[pipeline] 翻译工具已启用")
		}
	}

	// Home Assistant 智能家居工具
	if cfg.Tools.HomeAssistant.Enabled && cfg.Tools.HomeAssistant.URL != "" {
		haClient := tools.NewHomeAssistantClient(
			cfg.Tools.HomeAssistant.URL,
			cfg.Tools.HomeAssistant.Token,
		)
		p.toolRegistry.Register(tools.NewHAListDevicesTool(haClient))
		p.toolRegistry.Register(tools.NewHAGetDeviceStateTool(haClient))
		p.toolRegistry.Register(tools.NewHAControlDeviceTool(haClient))
		logger.Info("[pipeline] Home Assistant 智能家居工具已启用")
	}

	logger.Infof("[pipeline] 已注册 %d 个工具", p.toolRegistry.Count())
	return nil
}

// Run 启动主循环，阻塞直到 ctx 被取消。
func (p *Pipeline) Run(ctx context.Context) error {
	if err := p.capture.Start(); err != nil {
		return fmt.Errorf("启动音频采集失败: %w", err)
	}

	// 启动闹钟检查 goroutine
	go p.alarmChecker(ctx)

	logger.Info("[pipeline] 已启动 — 请说唤醒词开始对话！")

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
				logger.Infof("[pipeline] 闹钟到期: %s", a.Message)
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
	case StateSpeaking:
		// 播放期间检测唤醒词打断
		p.handleSpeakingInterrupt(ctx, frame)
	case StateProcessing:
		// 处理期间也检测唤醒词打断（消除句间盲区）
		p.handleProcessingInterrupt(ctx, frame)
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
		logger.Info("[pipeline] 检测到唤醒词！")

		// 进入冷却期，防止重复检测
		p.wakeCooldownMu.Lock()
		p.wakeCooldown = true
		p.wakeCooldownMu.Unlock()

		p.wakeDetector.Reset()
		p.vadDetector.Reset()
		p.recognizer.Reset()

		// 初始化声纹缓冲区（唤醒后开始收集音频）
		if p.voiceprintMgr != nil && p.voiceprintMgr.NumSpeakers() > 0 {
			p.voiceprintBufMu.Lock()
			p.voiceprintBuf = make([]float32, 0, p.voiceprintBufSize)
			p.voiceprintBufMu.Unlock()
		}

		// 如果配置了唤醒回复语，先播放再进入监听
		if p.cfg.Dialog.WakeReply != "" {
			p.state.Transition(StateSpeaking)
			go p.playWakeReply(ctx)
		} else {
			p.state.Transition(StateListening)
			// 启动连续对话超时计时器
			if p.cfg.Dialog.ContinuousTimeout > 0 {
				p.startContinuousTimer()
				logger.Infof("[pipeline] 进入连续对话模式，%d 秒内无输入将回到空闲", p.cfg.Dialog.ContinuousTimeout)
			}
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

// handleSpeakingInterrupt 在播放状态下检测唤醒词打断。
func (p *Pipeline) handleSpeakingInterrupt(ctx context.Context, frame []float32) {
	if p.detectWakeWord(frame) {
		logger.Info("[pipeline] 播放中检测到唤醒词，打断播放！")
		p.performInterrupt(ctx)
	}
}

// handleProcessingInterrupt 在处理状态下检测唤醒词打断（消除句间 TTS 合成盲区）。
func (p *Pipeline) handleProcessingInterrupt(ctx context.Context, frame []float32) {
	if p.detectWakeWord(frame) {
		logger.Info("[pipeline] 处理中检测到唤醒词，打断处理！")
		p.performInterrupt(ctx)
	}
}

// detectWakeWord 检测唤醒词（带冷却期检查）。
func (p *Pipeline) detectWakeWord(frame []float32) bool {
	p.wakeCooldownMu.Lock()
	if p.wakeCooldown {
		p.wakeCooldownMu.Unlock()
		return false
	}
	p.wakeCooldownMu.Unlock()

	return p.wakeDetector.Detect(frame)
}

// performInterrupt 执行打断逻辑：停止播放、取消 LLM 调用、设置打断标志、播放回复、延迟后进入监听。
func (p *Pipeline) performInterrupt(ctx context.Context) {
	// 进入冷却期
	p.wakeCooldownMu.Lock()
	p.wakeCooldown = true
	p.wakeCooldownMu.Unlock()

	p.wakeDetector.Reset()

	// 设置打断标志，通知 processQuery goroutine 退出
	p.interrupted.Store(true)

	// 取消 LLM 调用（如果正在进行）
	p.queryMu.Lock()
	if p.cancelQuery != nil {
		p.cancelQuery()
	}
	p.queryMu.Unlock()

	// 停止所有播放
	p.interruptSpeak()

	// 重置 ASR/VAD
	p.vadDetector.Reset()
	p.recognizer.Reset()

	// 播放打断回复语（区别于唤醒回复语）
	if p.cfg.Dialog.InterruptReply != "" {
		logger.Debugf("[pipeline] 播放打断回复: %s", p.cfg.Dialog.InterruptReply)
		p.speakText(ctx, p.cfg.Dialog.InterruptReply)
	}

	// 延迟后进入监听状态（给用户反应时间）
	if p.cfg.Dialog.ListenDelay > 0 {
		time.Sleep(time.Duration(p.cfg.Dialog.ListenDelay) * time.Millisecond)
	}
	p.capture.Drain() // 清空回声残留

	p.state.SetState(StateListening)

	// 启动连续对话超时计时器
	if p.cfg.Dialog.ContinuousTimeout > 0 {
		p.startContinuousTimer()
	}

	// 延迟解除冷却期
	time.AfterFunc(500*time.Millisecond, p.clearWakeCooldown)
}

// playWakeReply 播放唤醒回复语，完成后进入监听状态。
func (p *Pipeline) playWakeReply(ctx context.Context) {
	logger.Debugf("[pipeline] 播放唤醒回复: %s", p.cfg.Dialog.WakeReply)
	p.speakText(ctx, p.cfg.Dialog.WakeReply)

	// 延迟后进入监听状态（给用户反应时间）
	if p.cfg.Dialog.ListenDelay > 0 {
		time.Sleep(time.Duration(p.cfg.Dialog.ListenDelay) * time.Millisecond)
	}
	p.capture.Drain() // 清空回声残留

	// 播放完成后进入监听状态
	p.vadDetector.Reset()
	p.recognizer.Reset()
	p.state.SetState(StateListening)

	// 启动连续对话超时计时器
	if p.cfg.Dialog.ContinuousTimeout > 0 {
		p.startContinuousTimer()
		logger.Infof("[pipeline] 进入连续对话模式，%d 秒内无输入将回到空闲", p.cfg.Dialog.ContinuousTimeout)
	}

	// 解除冷却期（延迟一点，确保不会立即重复检测）
	time.AfterFunc(500*time.Millisecond, p.clearWakeCooldown)
}

// handleListening 同时将音频送入 VAD 和 ASR。
func (p *Pipeline) handleListening(ctx context.Context, frame []float32) {
	// 声纹缓冲：收集音频帧用于说话人识别
	p.voiceprintBufMu.Lock()
	if p.voiceprintBuf != nil && len(p.voiceprintBuf) < p.voiceprintBufSize {
		p.voiceprintBuf = append(p.voiceprintBuf, frame...)
		if len(p.voiceprintBuf) >= p.voiceprintBufSize {
			buf := p.voiceprintBuf
			p.voiceprintBuf = nil
			p.voiceprintBufMu.Unlock()
			p.voiceprintWg.Add(1)
			go func() {
				defer p.voiceprintWg.Done()
				p.identifySpeaker(buf)
			}()
		} else {
			p.voiceprintBufMu.Unlock()
		}
	} else {
		p.voiceprintBufMu.Unlock()
	}

	p.vadDetector.Feed(frame)
	p.recognizer.Feed(frame)

	text := p.recognizer.GetResult()
	if text != "" {
		logger.Debugf("[pipeline] 实时识别: %s", text)
		// ASR 有实时文本输出，说明有人在说话，重置超时计时器
		p.resetContinuousTimer()
	}

	if p.recognizer.IsEndpoint() {
		finalText := p.recognizer.GetResult()
		p.recognizer.Reset()
		p.vadDetector.Reset()

		// 如果声纹缓冲区还在收集且已有足够数据（>1秒），也触发识别
		p.voiceprintBufMu.Lock()
		if p.voiceprintBuf != nil && len(p.voiceprintBuf) > p.cfg.Audio.SampleRate {
			buf := p.voiceprintBuf
			p.voiceprintBuf = nil
			p.voiceprintBufMu.Unlock()
			p.voiceprintWg.Add(1)
			go func() {
				defer p.voiceprintWg.Done()
				p.identifySpeaker(buf)
			}()
		} else {
			p.voiceprintBufMu.Unlock()
		}

		if strings.TrimSpace(finalText) == "" {
			// ASR 结果为空，继续监听，等待超时计时器触发
			return
		}

		// 有有效文本，停止计时器，进入处理阶段
		p.stopContinuousTimer()

		logger.Infof("[pipeline] ASR 最终结果: %s", finalText)
		p.state.Transition(StateProcessing)
		go p.processQuery(ctx, finalText)
	}
}

// processQuery 将识别文本发送给 LLM，支持工具调用循环。
// 所有轮次先缓冲完整回复，再根据是否有工具调用决定处理方式：
//   - 有工具调用：丢弃前言文本，直接执行工具
//   - 无工具调用：合并短句后批量 TTS，减少合成次数
func (p *Pipeline) processQuery(ctx context.Context, query string) {
	// 等待声纹识别完成（如果正在进行）
	p.voiceprintWg.Wait()

	// 重置打断标志
	p.interrupted.Store(false)

	// 创建可取消的 sub-context，打断时可立即停止 LLM 调用
	queryCtx, cancelQuery := context.WithCancel(ctx)
	p.queryMu.Lock()
	p.cancelQuery = cancelQuery
	p.queryMu.Unlock()
	defer func() {
		cancelQuery()
		p.queryMu.Lock()
		p.cancelQuery = nil
		p.queryMu.Unlock()
	}()

	p.contextManager.Add("user", query)

	toolDefs := p.toolRegistry.Definitions()
	maxRounds := 5 // 最多 5 轮 LLM 调用（工具调用可能多轮，最后需要一轮生成回复）
	var lastHadToolCalls bool

	for round := 0; round < maxRounds; round++ {
		// 检查打断
		if p.interrupted.Load() {
			return
		}

		messages := p.contextManager.Messages()

		textCh, resultCh, err := p.llmProvider.ChatStreamWithTools(queryCtx, messages, toolDefs)
		if err != nil {
			logger.Errorf("[pipeline] LLM 调用失败: %v", err)
			// 使用备用 TTS 播放错误提示
			if p.fallbackTtsEngine != nil {
				p.state.SetState(StateSpeaking)
				p.speakText(ctx, "网络连接失败，请检查网络设置")
			}
			p.state.ForceIdle()
			return
		}

		// 先缓冲完整回复，等流结束后再决定处理方式
		var fullReply strings.Builder

		for chunk := range textCh {
			if p.interrupted.Load() {
				for range resultCh {
				}
				return
			}
			fullReply.WriteString(chunk)
		}

		// 获取最终结果（包含可能的 tool_calls）
		result := <-resultCh
		if result == nil {
			break
		}

		// 检查打断
		if p.interrupted.Load() {
			return
		}

		// 如果没有工具调用，合并短句后 TTS 播放
		if len(result.ToolCalls) == 0 {
			lastHadToolCalls = false
			replyText := strings.TrimSpace(fullReply.String())
			if replyText != "" && !p.interrupted.Load() {
				p.state.Transition(StateSpeaking)
				// 合并短句为大段（每段最多 100 个字符），减少 TTS 次数
				chunks := mergeSentences(replyText, 100)
				for _, chunk := range chunks {
					if chunk != "" && !p.interrupted.Load() {
						logger.Debugf("[pipeline] TTS 合成: %s", chunk)
						p.speakText(ctx, chunk)
					}
				}
			}
			p.contextManager.Add("assistant", fullReply.String())
			logger.Infof("[pipeline] LLM 回复完成 (%d 字符)", fullReply.Len())
			break
		}

		// 有工具调用 — 丢弃前言文本（如"我来帮你查询..."）
		lastHadToolCalls = true
		preamble := strings.TrimSpace(fullReply.String())
		if preamble != "" {
			logger.Debugf("[pipeline] 检测到工具调用，丢弃前言文本: %s", preamble)
		}

		// 切回 Processing，执行工具
		logger.Infof("[pipeline] 第 %d 轮工具调用: %d 个工具", round+1, len(result.ToolCalls))
		p.state.SetState(StateProcessing)

		// 将 assistant 消息（含 tool_calls）添加到上下文
		assistantMsg := llm.Message{
			Role:      "assistant",
			Content:   result.Content,
			ToolCalls: result.ToolCalls,
		}
		p.contextManager.AddMessage(assistantMsg)

		// 执行每个工具并将结果添加到上下文
		for _, tc := range result.ToolCalls {
			// 检查打断
			if p.interrupted.Load() {
				return
			}

			// 权限检查：声纹相关工具只有主人可用
			if isVoiceprintTool(tc.Function.Name) {
				speakerName := p.contextManager.GetCurrentSpeaker()
				if !p.voiceprintMgr.IsOwner(speakerName) {
					logger.Warnf("[pipeline] 非主人尝试调用 %s 工具: %s", tc.Function.Name, speakerName)
					p.contextManager.AddMessage(llm.Message{
						Role:       "tool",
						Content:    `{"success":false,"message":"此功能仅主人可用"}`,
						ToolCallID: tc.ID,
						Name:       tc.Function.Name,
					})
					continue
				}
			}

			logger.Infof("[pipeline] 调用工具: %s(%s)", tc.Function.Name, tc.Function.Arguments)

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
			if tc.Function.Name == "play_music" || tc.Function.Name == "next_music" {
				var musicResult tools.MusicResult
				if jsonErr := json.Unmarshal([]byte(toolResult), &musicResult); jsonErr == nil {
					if musicResult.Success && (musicResult.URL != "" || musicResult.CacheKey != "") {
						// 播放音乐
						logger.Infof("[pipeline] 开始播放音乐: %s - %s", musicResult.Artist, musicResult.SongName)
						p.playMusic(ctx, musicResult.URL, musicResult.CacheKey)
						// 音乐播放结束后继续
						return
					}
				}
			}
		}
		// 继续下一轮 LLM 调用
	}

	// 如果最后一轮仍有工具调用，说明达到最大轮数限制，可能未完成回复
	if lastHadToolCalls {
		logger.Warnf("[pipeline] 达到最大轮数 %d，可能未完成回复", maxRounds)
	}

	// 回复完成后进入连续对话模式（等待用户继续说）
	// 但如果已经被打断，则不进入
	if !p.interrupted.Load() {
		p.enterContinuousMode()
	}
}

// identifySpeaker 异步识别说话人并注入 LLM 上下文。
func (p *Pipeline) identifySpeaker(samples []float32) {
	if p.voiceprintMgr == nil {
		return
	}
	name, err := p.voiceprintMgr.Identify(samples)
	if err != nil {
		logger.Errorf("[pipeline] 声纹识别失败: %v", err)
		return
	}
	if name != "" {
		logger.Debugf("[pipeline] 声纹识别结果: %s", name)
		// 获取用户信息（包含偏好）
		user, err := p.voiceprintMgr.GetUser(name)
		if err != nil {
			logger.Warnf("[pipeline] 获取用户信息失败: %v", err)
			p.contextManager.SetCurrentSpeaker(name, nil)
		} else {
			p.contextManager.SetCurrentSpeaker(name, user)
		}
	} else {
		p.contextManager.SetCurrentSpeaker("", nil)
	}
}

// enterContinuousMode 进入连续对话模式。
// 回复完成后不立即回到空闲，而是进入监听状态并启动超时计时器。
func (p *Pipeline) enterContinuousMode() {
	// 清空声纹状态
	p.contextManager.SetCurrentSpeaker("", nil)
	p.voiceprintBufMu.Lock()
	p.voiceprintBuf = nil
	p.voiceprintBufMu.Unlock()

	if p.cfg.Dialog.ContinuousTimeout <= 0 {
		// 连续对话模式禁用，直接回到空闲
		p.state.ForceIdle()
		return
	}

	// 延迟 + 清空麦克风缓冲，防止扬声器回声被 ASR 识别
	if p.cfg.Dialog.ListenDelay > 0 {
		time.Sleep(time.Duration(p.cfg.Dialog.ListenDelay) * time.Millisecond)
	}
	p.capture.Drain()

	// 进入监听状态
	p.vadDetector.Reset()
	p.recognizer.Reset()
	p.state.ForceIdle() // 先重置
	p.state.Transition(StateListening)

	// 启动超时计时器
	p.startContinuousTimer()
	logger.Infof("[pipeline] 进入连续对话模式，%d 秒内无输入将回到空闲", p.cfg.Dialog.ContinuousTimeout)
}

// startContinuousTimer 启动连续对话超时计时器。
func (p *Pipeline) startContinuousTimer() {
	p.continuousMu.Lock()
	defer p.continuousMu.Unlock()

	// 停止之前的计时器
	if p.continuousTimer != nil {
		p.continuousTimer.Stop()
	}

	// 启动新计时器，超时后直接回到空闲
	p.continuousTimer = time.AfterFunc(time.Duration(p.cfg.Dialog.ContinuousTimeout)*time.Second, func() {
		if p.state.Current() == StateListening {
			logger.Info("[pipeline] 连续对话超时，回到空闲状态")
			p.state.ForceIdle()
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
	// 重新启动计时器（相当于重置超时时间）
	p.startContinuousTimer()
}

// speakText 合成并播放单段文本。
// 如果主 TTS 引擎失败且有备用引擎，则使用备用引擎播放错误提示。
func (p *Pipeline) speakText(ctx context.Context, text string) {
	samples, sampleRate, err := p.ttsEngine.Synthesize(ctx, text)
	if err != nil {
		logger.Errorf("[pipeline] TTS 合成失败: %v", err)
		// 尝试使用备用引擎播放错误提示
		if p.fallbackTtsEngine != nil {
			fallbackText := "语音合成失败，请检查网络连接"
			if fbSamples, fbRate, fbErr := p.fallbackTtsEngine.Synthesize(ctx, fallbackText); fbErr == nil && len(fbSamples) > 0 {
				logger.Info("[pipeline] 使用备用 TTS 引擎播放错误提示")
				p.playSamples(ctx, fbSamples, fbRate)
			} else if fbErr != nil {
				logger.Errorf("[pipeline] 备用 TTS 也失败: %v", fbErr)
			}
		}
		return
	}
	if len(samples) == 0 {
		return
	}

	p.playSamples(ctx, samples, sampleRate)
}

// playSamples 播放音频样本。
func (p *Pipeline) playSamples(ctx context.Context, samples []float32, sampleRate int) {
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
		logger.Errorf("[pipeline] 播放失败: %v", err)
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

// playMusic 播放音乐，播放结束后自动播放列表中的下一首。
func (p *Pipeline) playMusic(ctx context.Context, url string, cacheKey string) {
	// 确保状态为 Speaking
	if p.state.Current() != StateSpeaking {
		p.state.SetState(StateSpeaking)
	}

	// 构建播放选项
	var opts *audio.PlayOptions
	if cacheKey != "" && p.musicCache != nil {
		opts = &audio.PlayOptions{
			CacheKey: cacheKey,
			Cache:    p.musicCache,
		}
	}

	if err := p.streamPlayer.Play(ctx, url, opts); err != nil {
		if err != context.Canceled {
			logger.Errorf("[pipeline] 音乐播放失败: %v", err)
		}
		// 被打断或出错，不自动下一首
		p.enterContinuousMode()
		return
	}

	// 播放完成，更新缓存索引（如果走了网络下载路径）
	if opts != nil && p.musicCache != nil && p.musicCache.Enabled() && cacheKey != "" {
		// 检查缓存文件是否存在（下载完成后会 commit）
		filePath := p.musicCache.FilePath(cacheKey)
		if _, err := os.Stat(filePath); err == nil {
			// 从 playlist 获取当前歌曲信息来更新索引
			if item := p.playlist.Current(); item != nil {
				p.musicCache.Store(cacheKey, audio.CacheEntry{
					ID:       item.Song.ID,
					Name:     item.Song.Name,
					Artist:   item.Song.Artist,
					Album:    item.Song.Album,
					Provider: cacheKey[:strings.Index(cacheKey, "_")],
				})
			}
		}
	}

	// 播放正常完成，尝试自动播放下一首
	if p.playlist != nil && p.playlist.HasNext() {
		nextURL, songName, artist, nextCacheKey, ok := p.playlist.Next(ctx)
		if ok {
			logger.Infof("[pipeline] 自动切换下一首: %s - %s", artist, songName)
			// 递归播放下一首（仍在同一个 goroutine 中）
			p.playMusic(ctx, nextURL, nextCacheKey)
			return
		}
	}

	// 列表播完或无下一首，进入连续对话模式
	logger.Info("[pipeline] 播放列表结束")
	p.enterContinuousMode()
}

// Close 释放所有资源。
func (p *Pipeline) Close() {
	logger.Info("[pipeline] 正在关闭...")

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
	if p.voiceprintMgr != nil {
		p.voiceprintMgr.Close()
	}

	logger.Info("[pipeline] 已关闭")
}

// isVoiceprintTool 检查是否是声纹相关工具（仅主人可用）。
func isVoiceprintTool(name string) bool {
	switch name {
	case "register_voiceprint", "delete_voiceprint", "set_user_preferences":
		return true
	default:
		return false
	}
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

// mergeSentences 将文本按句分割后合并为大段，每段不超过 maxChars 个字符。
// 腾讯云 TTS 单次最大约 150 字符（中文），这里按 100 字符合并以留余量。
func mergeSentences(text string, maxChars int) []string {
	if maxChars <= 0 {
		maxChars = 100
	}

	var chunks []string
	var current strings.Builder
	remaining := text

	flush := func() {
		s := strings.TrimSpace(current.String())
		if s != "" {
			chunks = append(chunks, s)
		}
		current.Reset()
	}

	for {
		sentence, rest, found := extractSentence(remaining)
		if !found {
			if r := strings.TrimSpace(remaining); r != "" {
				// 如果追加后超限，先刷出
				if current.Len() > 0 && utf8.RuneCountInString(current.String())+utf8.RuneCountInString(r) > maxChars {
					flush()
				}
				current.WriteString(r)
			}
			break
		}
		remaining = rest
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		sentenceLen := utf8.RuneCountInString(sentence)
		currentLen := utf8.RuneCountInString(current.String())

		// 如果当前段追加后超限，先刷出当前段
		if current.Len() > 0 && currentLen+sentenceLen > maxChars {
			flush()
		}
		current.WriteString(sentence)
	}
	flush()
	return chunks
}
