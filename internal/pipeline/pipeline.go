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
	"github.com/iabetor/pibuddy/internal/database"
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
	db  *database.DB // 统一数据库

	capture *audio.Capture
	player  *audio.Player

	wakeDetector *wake.Detector
	vadDetector  *vad.Detector
	recognizer   asr.Engine // ASR 引擎（支持多引擎兜底）

	llmProvider    llm.Provider
	contextManager *llm.ContextManager

	ttsEngine         tts.Engine
	fallbackTtsEngine tts.Engine // 回退 TTS 引擎（网络失败时使用）

	toolRegistry *tools.Registry
	alarmStore   *tools.AlarmStore
	timerStore   *tools.TimerStore
	volumeCtrl   tools.VolumeController
	healthStore  *tools.HealthStore

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
	wakeCooldown   bool // 是否处于冷却期
	wakeCooldownMu sync.Mutex

	// 回声静默期：打断后的静默期内丢弃所有音频帧
	echoSilenceUntil time.Time
	echoSilenceMu    sync.Mutex

	// 打断标志（跨 goroutine 通信，通知 processQuery 退出）
	interrupted atomic.Bool

	// 声纹识别
	voiceprintMgr     *voiceprint.Manager
	voiceprintBuf     []float32
	voiceprintBufMu   sync.Mutex
	voiceprintBufSize int            // 目标缓冲大小 = BufferSecs * SampleRate
	voiceprintWg      sync.WaitGroup // 等待声纹识别完成

	// 暂停的音乐存储（用于恢复播放）
	pausedStore *music.PausedMusicStore

	// 音乐播放时间跟踪
	musicPlayStart    time.Time // 当前歌曲播放开始时间
	musicPlayStartMu  sync.Mutex
	currentCacheKey   string // 当前歌曲的缓存 key

	// 收藏存储
	favoritesStore *music.FavoritesStore

	// ASR 中间结果去重（只在变化时打印日志）
	lastASRText string
}

// New 根据配置创建并初始化完整的 Pipeline。
func New(cfg *config.Config) (*Pipeline, error) {
	p := &Pipeline{
		cfg:   cfg,
		state: NewStateMachine(),
	}

	var err error

	// 初始化统一数据库
	p.db, err = database.Open("")
	if err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}
	if err := p.db.Migrate(); err != nil {
		p.Close()
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

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

	// 流式语音识别（支持多引擎兜底）
	p.recognizer, err = initASREngine(cfg)
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

	// 初始化声纹识别（可选，失败不阻止启动）— 必须在 initTools 之前，工具注册需要 voiceprintMgr
	logger.Debugf("[pipeline] 声纹配置: enabled=%v, model=%s", cfg.Voiceprint.Enabled, cfg.Voiceprint.ModelPath)
	if cfg.Voiceprint.Enabled && cfg.Voiceprint.ModelPath != "" {
		vpMgr, vpErr := voiceprint.NewManager(cfg.Voiceprint, cfg.Tools.DataDir)
		if vpErr != nil {
			logger.Warnf("[pipeline] 声纹识别初始化失败（已禁用）: %v", vpErr)
		} else {
			p.voiceprintMgr = vpMgr
			p.voiceprintBufSize = int(cfg.Voiceprint.BufferSecs * float32(cfg.Audio.SampleRate))
			logger.Infof("[pipeline] 声纹识别已启用，已注册 %d 个用户", vpMgr.NumSpeakers())

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

	// 初始化流式播放器（音乐）— 必须在 initTools 之前，工具注册需要 streamPlayer
	streamPlayer, err := audio.NewStreamPlayer(1) // 单声道
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("初始化流式播放器失败: %w", err)
	}
	p.streamPlayer = streamPlayer

	// 初始化工具（需要 voiceprintMgr 已就绪）
	if err := p.initTools(cfg); err != nil {
		p.Close()
		return nil, fmt.Errorf("初始化工具失败: %w", err)
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
		musicCache, err = audio.NewMusicCache(p.db, cfg.Tools.Music.CacheDir, cfg.Tools.Music.CacheMaxSize)
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

		// 初始化收藏存储
		p.favoritesStore = music.NewFavoritesStore(cfg.Tools.DataDir)

		// 收藏和恢复播放工具
		favCfg := tools.FavoritesConfig{
			Store:          p.favoritesStore,
			Playlist:       p.playlist,
			ContextManager: p.contextManager,
		}
		p.toolRegistry.Register(tools.NewAddFavoriteTool(favCfg))
		p.toolRegistry.Register(tools.NewRemoveFavoriteTool(favCfg))
		p.toolRegistry.Register(tools.NewListFavoritesTool(favCfg))
		p.toolRegistry.Register(tools.NewPlayFavoritesTool(favCfg, musicProvider))

		// 恢复播放工具
		p.pausedStore = music.NewPausedMusicStore()
		p.toolRegistry.Register(tools.NewResumeMusicTool(p.playlist, p.pausedStore, musicCache))
		p.toolRegistry.Register(tools.NewStopMusicTool(p.playlist, p.pausedStore))
		logger.Info("[pipeline] 音乐收藏和恢复播放工具已启用")
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

	// whoami 和 list_voiceprint_users 始终注册（即使声纹未启用，返回友好提示）
	p.toolRegistry.Register(tools.NewWhoAmITool(p.voiceprintMgr, p.contextManager))
	p.toolRegistry.Register(tools.NewListVoiceprintUsersTool(p.voiceprintMgr))

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

	// 休息工具
	p.toolRegistry.Register(tools.NewGoToSleepTool())

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

	// 萤石门锁工具
	if cfg.Tools.Ezviz.Enabled && cfg.Tools.Ezviz.AppKey != "" {
		ezvizClient := tools.NewEzvizClient(cfg.Tools.Ezviz.AppKey, cfg.Tools.Ezviz.AppSecret)
		p.toolRegistry.Register(tools.NewEzvizListDevicesTool(ezvizClient))
		p.toolRegistry.Register(tools.NewEzvizGetLockStatusTool(ezvizClient, cfg.Tools.Ezviz.DeviceSerial))
		p.toolRegistry.Register(tools.NewEzvizOpenDoorTool(ezvizClient, cfg.Tools.Ezviz.DeviceSerial))
		logger.Info("[pipeline] 萤石门锁工具已启用")
	}

	// 系统状态工具
	p.toolRegistry.Register(tools.NewSystemStatusTool())

	// 健康提醒工具
	if cfg.Tools.Health.Enabled {
		healthStore, err := tools.NewHealthStore(cfg.Tools.DataDir, tools.HealthStoreConfig{
			WaterInterval:    cfg.Tools.Health.WaterInterval,
			ExerciseInterval: cfg.Tools.Health.ExerciseInterval,
			QuietHoursStart:  cfg.Tools.Health.QuietHours.Start,
			QuietHoursEnd:    cfg.Tools.Health.QuietHours.End,
		})
		if err != nil {
			return fmt.Errorf("初始化健康提醒存储失败: %w", err)
		}
		p.healthStore = healthStore
		p.toolRegistry.Register(tools.NewSetHealthReminderTool(healthStore))
		p.toolRegistry.Register(tools.NewListHealthRemindersTool(healthStore))
		logger.Info("[pipeline] 健康提醒工具已启用")
	}

	// 学习工具
	if cfg.Tools.Learning.Enabled {
		// 拼音工具（本地库，无需配置）
		p.toolRegistry.Register(tools.NewPinyinTool())

		// 英语学习工具
		if cfg.Tools.Learning.English.Enabled {
			p.toolRegistry.Register(tools.NewEnglishWordTool())
			p.toolRegistry.Register(tools.NewEnglishDailyTool())
			p.toolRegistry.Register(tools.NewVocabularyTool(cfg.Tools.DataDir))
			p.toolRegistry.Register(tools.NewEnglishQuizTool(cfg.Tools.DataDir))
			logger.Info("[pipeline] 英语学习工具已启用")
		}

		// 古诗词工具
		if cfg.Tools.Learning.Poetry.Enabled {
			p.toolRegistry.Register(tools.NewPoetryDailyTool(cfg.Tools.Learning.Poetry.APIKey))
			p.toolRegistry.Register(tools.NewPoetrySearchTool(cfg.Tools.Learning.Poetry.APIKey))
			p.toolRegistry.Register(tools.NewPoetryGameTool(cfg.Tools.Learning.Poetry.APIKey))
			logger.Info("[pipeline] 古诗词工具已启用")
		}
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

	// 启动健康提醒检查 goroutine
	if p.healthStore != nil {
		go p.healthReminderChecker(ctx)
	}

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

// healthReminderChecker 每分钟检查一次健康提醒。
func (p *Pipeline) healthReminderChecker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if p.healthStore == nil {
				continue
			}
			reminders := p.healthStore.CheckAndTrigger()
			for _, r := range reminders {
				logger.Infof("[pipeline] 健康提醒: %s", r.Message)
				p.speakText(ctx, r.Message)
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

	// 立即清空麦克风缓冲（防止音乐残留）
	p.capture.Drain()

	// 重置 ASR/VAD
	p.vadDetector.Reset()
	p.recognizer.Reset()

	// 播放打断回复语（区别于唤醒回复语）
	if p.cfg.Dialog.InterruptReply != "" {
		logger.Debugf("[pipeline] 播放打断回复: %s", p.cfg.Dialog.InterruptReply)
		p.speakText(ctx, p.cfg.Dialog.InterruptReply)
	}

	// 延迟后进入监听状态（给用户反应时间 + 让回声消散）
	if p.cfg.Dialog.ListenDelay > 0 {
		time.Sleep(time.Duration(p.cfg.Dialog.ListenDelay) * time.Millisecond)
	}
	// 再次清空缓冲（播放"我在"期间的回声）
	p.capture.Drain()
	// 音乐播放后的回声需要更长时间消散，额外等待
	time.Sleep(100 * time.Millisecond)
	p.capture.Drain()
	// 最后再重置一次 VAD/ASR，确保没有残留状态
	p.vadDetector.Reset()
	p.recognizer.Reset()

	// 缩短静默期，避免截断用户说话
	p.echoSilenceMu.Lock()
	p.echoSilenceUntil = time.Now().Add(200 * time.Millisecond)
	p.echoSilenceMu.Unlock()

	p.state.SetState(StateListening)

	// 启动连续对话超时计时器
	if p.cfg.Dialog.ContinuousTimeout > 0 {
		p.startContinuousTimer()
	}

	// 延迟解除冷却期
	time.AfterFunc(300*time.Millisecond, p.clearWakeCooldown)
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
	// 检查是否在静默期内（打断后的回声消散期）
	p.echoSilenceMu.Lock()
	silenceUntil := p.echoSilenceUntil
	p.echoSilenceMu.Unlock()
	if time.Now().Before(silenceUntil) {
		// 静默期内丢弃帧，不送入 VAD/ASR
		return
	}

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
		// 只在中间结果变化时打印日志，避免相同结果重复刷屏
		if text != p.lastASRText {
			logger.Debugf("[pipeline] 实时识别: %s", text)
			p.lastASRText = text
		}
		// ASR 有实时文本输出，说明有人在说话，重置超时计时器
		p.resetContinuousTimer()
	}

	if p.recognizer.IsEndpoint() {
		finalText := p.recognizer.GetResult()
		p.recognizer.Reset()
		p.lastASRText = "" // 清除中间结果去重状态
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

		// 清理 ASR 结果中的杂音
		finalText = sanitizeASRText(finalText)
		// 纠正常见的同音字错误
		finalText = correctASRMistakes(finalText)
		if finalText == "" {
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
			// 检查是否为余额不足错误
			if llm.IsInsufficientBalance(err) {
				p.state.SetState(StateSpeaking)
				p.speakTextWithFallback(ctx, "大模型余额不足，请充值后再试")
			} else if p.fallbackTtsEngine != nil {
				// 使用备用 TTS 播放错误提示
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
						logger.Infof("[小派] %s", chunk)
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
			if tc.Function.Name == "play_music" || tc.Function.Name == "next_music" || tc.Function.Name == "resume_music" {
				var musicResult tools.MusicResult
				if jsonErr := json.Unmarshal([]byte(toolResult), &musicResult); jsonErr == nil {
					if musicResult.Success && (musicResult.URL != "" || musicResult.CacheKey != "") {
						// 播放音乐
						logger.Infof("[pipeline] 开始播放音乐: %s - %s", musicResult.Artist, musicResult.SongName)
						p.playMusicFromPosition(ctx, musicResult.URL, musicResult.CacheKey, musicResult.PositionSec)
						// 音乐播放结束后继续
						return
					}
				}
			}

			// 检查是否是休息命令
			if tc.Function.Name == "go_to_sleep" {
				var sleepResult struct {
					Success bool   `json:"success"`
					Action  string `json:"action"`
					Message string `json:"message"`
				}
				if jsonErr := json.Unmarshal([]byte(toolResult), &sleepResult); jsonErr == nil {
					if sleepResult.Success && sleepResult.Action == "sleep" {
						logger.Info("[pipeline] 用户说休息，停止监听")
						// 停止连续对话计时器
						p.stopContinuousTimer()
						// 直接回到空闲状态
						p.state.ForceIdle()
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
	// 清空声纹状态，但重新初始化缓冲区（为下一次对话准备）
	p.contextManager.SetCurrentSpeaker("", nil)
	if p.voiceprintMgr != nil && p.voiceprintMgr.NumSpeakers() > 0 {
		p.voiceprintBufMu.Lock()
		p.voiceprintBuf = make([]float32, 0, p.voiceprintBufSize)
		p.voiceprintBufMu.Unlock()
	}

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
	p.speakTextWithFallback(ctx, text)
}

// speakTextWithFallback 使用主 TTS 引擎合成并播放文本，失败时使用备用引擎。
// 如果主引擎是余额不足错误，使用备用引擎播放提示信息。
func (p *Pipeline) speakTextWithFallback(ctx context.Context, text string) {
	samples, sampleRate, err := p.ttsEngine.Synthesize(ctx, text)
	if err != nil {
		logger.Errorf("[pipeline] TTS 合成失败: %v", err)
		// 尝试使用备用引擎
		if p.fallbackTtsEngine != nil {
			// 检查是否为余额不足错误
			fallbackText := "语音合成失败，请检查网络连接"
			if tts.IsInsufficientBalance(err) {
				fallbackText = "语音合成余额不足，请充值后再试"
			}
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

	// 暂停音乐播放并保存状态
	if p.streamPlayer != nil {
		p.streamPlayer.Stop()
	}

	// 保存当前播放状态（用于恢复播放）
	p.savePausedMusic()
}

// savePausedMusic 保存当前播放状态。
func (p *Pipeline) savePausedMusic() {
	if p.playlist == nil {
		return
	}

	current := p.playlist.Current()
	if current == nil {
		return
	}

	// 计算播放位置
	p.musicPlayStartMu.Lock()
	positionSec := time.Since(p.musicPlayStart).Seconds()
	cacheKey := p.currentCacheKey
	p.musicPlayStartMu.Unlock()

	p.pausedStore.Save(
		p.playlist.GetItems(),
		p.playlist.CurrentIndex(),
		p.playlist.Mode(),
		current.Song.Name,
		positionSec,
		cacheKey,
	)

	logger.Infof("[pipeline] 已保存播放状态: %s (索引 %d/%d, 位置 %.1fs)",
		current.Song.Name, p.playlist.CurrentIndex()+1, p.playlist.Len(), positionSec)
}

// playMusic 播放音乐，播放结束后自动播放列表中的下一首。
func (p *Pipeline) playMusic(ctx context.Context, url string, cacheKey string) {
	p.playMusicFromPosition(ctx, url, cacheKey, 0)
}

// playMusicFromPosition 从指定位置播放音乐，播放结束后自动播放列表中的下一首。
// positionSec > 0 时，如果缓存存在则从指定位置开始播放。
func (p *Pipeline) playMusicFromPosition(ctx context.Context, url string, cacheKey string, positionSec float64) {
	// 确保状态为 Speaking
	if p.state.Current() != StateSpeaking {
		p.state.SetState(StateSpeaking)
	}

	// 记录播放开始时间和缓存 key（用于恢复播放）
	// 如果从位置恢复，需要调整开始时间以反映实际播放位置
	p.musicPlayStartMu.Lock()
	if positionSec > 0 {
		p.musicPlayStart = time.Now().Add(-time.Duration(positionSec * float64(time.Second)))
	} else {
		p.musicPlayStart = time.Now()
	}
	p.currentCacheKey = cacheKey
	p.musicPlayStartMu.Unlock()

	// 检查是否可以从缓存文件的位置播放
	if positionSec > 0 && cacheKey != "" && p.musicCache != nil {
		if cachedPath, ok := p.musicCache.Lookup(cacheKey); ok {
			logger.Infof("[pipeline] 从 %.0f 秒处恢复播放 (缓存: %s)", positionSec, cacheKey)
			actualPos, err := p.streamPlayer.PlayFromPosition(ctx, cachedPath, positionSec)
			if err != nil {
				logger.Warnf("[pipeline] 从位置播放失败，从头播放: %v", err)
				// 失败时从头播放
				positionSec = 0
				p.musicPlayStartMu.Lock()
				p.musicPlayStart = time.Now()
				p.musicPlayStartMu.Unlock()
				opts := &audio.PlayOptions{
					CacheKey: cacheKey,
					Cache:    p.musicCache,
				}
				if err := p.streamPlayer.Play(ctx, url, opts); err != nil {
					if err != context.Canceled {
						logger.Errorf("[pipeline] 音乐播放失败: %v", err)
					}
					p.enterContinuousMode()
					return
				}
			} else {
				logger.Infof("[pipeline] 实际从 %.0f 秒开始播放", actualPos)
			}
			// 播放完成，处理下一首
			p.handleMusicCompletion(ctx, cacheKey)
			return
		}
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

	// 播放完成，处理下一首
	p.handleMusicCompletion(ctx, cacheKey)
}

// handleMusicCompletion 处理音乐播放完成后的逻辑（更新缓存索引、自动下一首）。
func (p *Pipeline) handleMusicCompletion(ctx context.Context, cacheKey string) {
	// 播放完成，更新缓存索引（如果走了网络下载路径）
	if cacheKey != "" && p.musicCache != nil && p.musicCache.Enabled() {
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
	if p.db != nil {
		p.db.Close()
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

// sanitizeASRText 清理 ASR 结果中的常见杂音和误识别。
// 例如 "SPK播放音乐" -> "播放音乐"
func sanitizeASRText(text string) string {
	text = strings.TrimSpace(text)

	// 常见的 ASR 杂音前缀模式
	noisePrefixes := []string{
		"SPK",    // speaker 标记误识别
		"SPK0",   // speaker 编号
		"SPK1",
		"SPK2",
		"spk",    // 小写形式
		"Spk",
		"SKP",    // 可能的变体
		"S P K",  // 分开的字母
	}

	for _, prefix := range noisePrefixes {
		if strings.HasPrefix(text, prefix) {
			// 移除前缀及后续可能的空格或标点
			rest := strings.TrimPrefix(text, prefix)
			rest = strings.TrimLeft(rest, " 　,，.。:：!！?？")
			if rest != "" {
				text = rest
				break
			}
		}
	}

	// 移除开头的纯字母杂音（如单独的 "A", "B" 等，后跟中文）
	// 但保留正常的英文单词
	if len(text) > 1 {
		// 检查开头是否为 1-3 个大写字母后跟中文
		for i := 1; i <= 3 && i < len(text); i++ {
			prefix := text[:i]
			if len(prefix) > 0 && prefix[0] >= 'A' && prefix[0] <= 'Z' {
				allUpper := true
				for _, c := range prefix {
					if c < 'A' || c > 'Z' {
						allUpper = false
						break
					}
				}
				if allUpper && i < len(text) {
					// 检查下一个字符是否为中文
					nextRune, _ := utf8.DecodeRuneInString(text[i:])
					if nextRune >= 0x4E00 && nextRune <= 0x9FFF {
						// 是中文，检查这个前缀是否像杂音
						// 单个字母或 SPK 模式更可能是杂音
						if i <= 2 {
							rest := strings.TrimLeft(text[i:], " 　,，.。:：!！?？")
							if rest != "" {
								text = rest
								break
							}
						}
					}
				}
			}
		}
	}

	return strings.TrimSpace(text)
}

// correctASRMistakes 纠正 ASR 的常见同音字错误。
// 主要针对歌曲名、人名、常用词等进行纠正。
func correctASRMistakes(text string) string {
	// 纠错映射表：错误 -> 正确
	// 按歌曲名、人名、常用词分类
	corrections := map[string]string{
		// 歌曲名纠错
		"断桥残学": "断桥残雪", // 许嵩歌曲
		"断桥残血": "断桥残雪",
		"清明雨上": "清明雨上", // 保持正确
		"清明雨伤": "清明雨上",
		"有何不可": "有何不可", // 保持正确
		"有何不渴": "有何不可",
		"灰色头像": "灰色头像", // 保持正确
		"灰色偷像": "灰色头像",
		"千百度":   "千百度", // 保持正确
		"千百肚":   "千百度",

		// 歌手名纠错
		"许松": "许嵩",
		"许菘": "许嵩",
		"周杰伦": "周杰伦", // 保持正确
		"周杰轮": "周杰伦",
		"林俊杰": "林俊杰", // 保持正确
		"林俊节": "林俊杰",
		"邓紫棋": "邓紫棋", // 保持正确
		"邓子棋": "邓紫棋",
		"薛之谦": "薛之谦", // 保持正确
		"薛志谦": "薛之谦",

		// 常用词纠错
		"播放": "播放", // 保持正确
		"拨放": "播放",
		"暂停": "暂停", // 保持正确
		"暂廷": "暂停",
	}

	for wrong, correct := range corrections {
		if wrong != correct {
			text = strings.ReplaceAll(text, wrong, correct)
		}
	}

	return text
}

// initASREngine 初始化 ASR 引擎，支持多引擎兜底。
// 按 asr.priority 列表中的顺序初始化引擎，额度用完自动切换到下一个。
// sherpa 始终作为最终兜底引擎（端点检测 + 离线识别）。
func initASREngine(cfg *config.Config) (asr.Engine, error) {
	var engines []asr.Engine
	var engineTypes []asr.EngineType

	// 获取腾讯云密钥（优先使用 ASR 配置，其次复用 TTS 配置）
	secretID := cfg.ASR.Tencent.SecretID
	secretKey := cfg.ASR.Tencent.SecretKey
	if secretID == "" {
		secretID = cfg.TTS.Tencent.SecretID
	}
	if secretKey == "" {
		secretKey = cfg.TTS.Tencent.SecretKey
	}

	// 按优先级列表初始化引擎
	logger.Infof("[pipeline] ASR 引擎优先级: %v", cfg.ASR.Priority)
	for _, name := range cfg.ASR.Priority {
		switch name {
		case "tencent-flash":
			if secretID == "" || secretKey == "" {
				logger.Warn("[pipeline] 未配置腾讯云密钥，跳过腾讯云一句话识别引擎")
				continue
			}
			engine, err := asr.NewTencentFlashEngine(asr.TencentFlashConfig{
				SecretID:  secretID,
				SecretKey: secretKey,
				Region:    cfg.ASR.Tencent.Region,
			})
			if err != nil {
				logger.Warnf("[pipeline] 腾讯云一句话识别引擎初始化失败: %v", err)
				continue
			}
			engines = append(engines, engine)
			engineTypes = append(engineTypes, asr.EngineTencentFlash)

		case "tencent-rt":
			if secretID == "" || secretKey == "" {
				logger.Warn("[pipeline] 未配置腾讯云密钥，跳过腾讯云实时语音识别引擎")
				continue
			}
			if cfg.ASR.Tencent.AppID == "" {
				logger.Warn("[pipeline] 未配置腾讯云 AppID，跳过腾讯云实时语音识别引擎")
				continue
			}
			engine, err := asr.NewTencentRTEngine(asr.TencentRTConfig{
				SecretID:  secretID,
				SecretKey: secretKey,
				Region:    cfg.ASR.Tencent.Region,
				AppID:     cfg.ASR.Tencent.AppID,
			})
			if err != nil {
				logger.Warnf("[pipeline] 腾讯云实时语音识别引擎初始化失败: %v", err)
				continue
			}
			engines = append(engines, engine)
			engineTypes = append(engineTypes, asr.EngineTencentRT)

		case "sherpa":
			if cfg.ASR.ModelPath == "" {
				logger.Warn("[pipeline] 未配置 ASR 模型路径，跳过 Sherpa 引擎")
				continue
			}
			engine, err := asr.NewSherpaEngine(
				cfg.ASR.ModelPath,
				cfg.ASR.NumThreads,
				cfg.ASR.Rule1MinTrailingSilence,
				cfg.ASR.Rule2MinTrailingSilence,
				cfg.ASR.Rule3MinUtteranceLength,
			)
			if err != nil {
				logger.Warnf("[pipeline] Sherpa 引擎初始化失败: %v", err)
				continue
			}
			engines = append(engines, engine)
			engineTypes = append(engineTypes, asr.EngineSherpa)

		default:
			logger.Warnf("[pipeline] 未知的 ASR 引擎类型: %s，跳过", name)
		}
	}

	// 如果没有可用引擎，返回错误
	if len(engines) == 0 {
		return nil, fmt.Errorf("没有可用的 ASR 引擎，请检查配置")
	}

	// 如果只有一个引擎，直接返回
	if len(engines) == 1 {
		return engines[0], nil
	}

	// 多引擎：创建兜底引擎
	return asr.NewFallbackEngine(asr.FallbackConfig{
		Engines:     engines,
		EngineTypes: engineTypes,
	}), nil
}
