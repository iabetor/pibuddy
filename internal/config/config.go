package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 是 PiBuddy 的顶层配置结构。
type Config struct {
	Audio          AudioConfig    `yaml:"audio"`
	Wake           WakeConfig     `yaml:"wake"`
	VAD            VADConfig      `yaml:"vad"`
	ASR            ASRConfig      `yaml:"asr"`
	LLM            LLMConfig      `yaml:"llm"`
	TTS            TTSConfig      `yaml:"tts"`
	Tools          ToolsConfig    `yaml:"tools"`
	Log            LogConfig      `yaml:"log"`
	Dialog         DialogConfig     `yaml:"dialog"`
	Voiceprint     VoiceprintConfig `yaml:"voiceprint"`
}

// DialogConfig 对话配置。
type DialogConfig struct {
	// ContinuousTimeout 连续对话超时时间（秒）。
	// 回复完成后等待用户继续说话的时间，超过此时间回到空闲状态。
	// 设为 0 禁用连续对话模式。
	ContinuousTimeout int `yaml:"continuous_timeout"`

	// WakeReply 唤醒词触发后的回复语。
	// 为空则不播放回复语，直接进入监听状态。
	WakeReply string `yaml:"wake_reply"`

	// InterruptReply 播放被打断时的回复语。
	// 在播放中检测到唤醒词打断时播放，为空则不播放直接进入监听。
	InterruptReply string `yaml:"interrupt_reply"`

	// ListenDelay 播放回复语后延迟进入监听的时间（毫秒）。
	// 给用户一点反应时间再开始监听，默认 500ms。
	ListenDelay int `yaml:"listen_delay"`
}

// VoiceprintConfig 声纹识别配置。
type VoiceprintConfig struct {
	Enabled    bool    `yaml:"enabled"`
	ModelPath  string  `yaml:"model_path"`
	Threshold  float32 `yaml:"threshold"`
	NumThreads int     `yaml:"num_threads"`
	BufferSecs float32 `yaml:"buffer_secs"`
	OwnerName  string  `yaml:"owner_name"` // 主人姓名
}

// AudioConfig 音频采集/播放配置。
type AudioConfig struct {
	SampleRate int `yaml:"sample_rate"`
	Channels   int `yaml:"channels"`
	FrameSize  int `yaml:"frame_size"`
}

// WakeConfig 唤醒词检测配置。
type WakeConfig struct {
	ModelPath    string  `yaml:"model_path"`
	KeywordsFile string  `yaml:"keywords_file"`
	Threshold    float32 `yaml:"threshold"`
}

// VADConfig 语音活动检测配置。
type VADConfig struct {
	ModelPath    string  `yaml:"model_path"`
	Threshold    float32 `yaml:"threshold"`
	MinSilenceMs int    `yaml:"min_silence_ms"`
}

// ASRConfig 语音识别配置。
type ASRConfig struct {
	// Priority 引擎优先级列表，按顺序尝试，额度用完自动切换到下一个。
	// 可选值：tencent-flash（腾讯云一句话）、tencent-rt（腾讯云实时）、sherpa（离线）
	// 默认为 ["tencent-flash", "tencent-rt", "sherpa"]
	// sherpa 始终作为最终兜底，即使未列出也会自动添加。
	Priority []string `yaml:"priority"`

	// Provider 主引擎类型（兼容旧配置，优先使用 priority）
	Provider string `yaml:"provider"`

	// Fallback 兜底引擎（兼容旧配置，优先使用 priority）
	Fallback string `yaml:"fallback"`

	// 离线引擎配置（sherpa-onnx）
	ModelPath              string  `yaml:"model_path"`
	NumThreads             int     `yaml:"num_threads"`
	Rule1MinTrailingSilence float64 `yaml:"rule1_min_trailing_silence"` // 尾部静音阈值（秒）
	Rule2MinTrailingSilence float64 `yaml:"rule2_min_trailing_silence"` // 尾部静音阈值（秒）
	Rule3MinUtteranceLength float64 `yaml:"rule3_min_utterance_length"` // 最小语音长度（秒）

	// 腾讯云配置（可复用 TTS 的密钥）
	Tencent ASRTencentConfig `yaml:"tencent"`
}

// ASRTencentConfig 腾讯云 ASR 配置。
type ASRTencentConfig struct {
	SecretID  string `yaml:"secret_id"`
	SecretKey string `yaml:"secret_key"`
	Region    string `yaml:"region"`  // 默认 ap-guangzhou
	AppID     string `yaml:"app_id"`  // 实时语音识别需要
}

// LLMModelConfig 单个 LLM 模型配置。
type LLMModelConfig struct {
	Name   string `yaml:"name"`    // 显示名称，如 "qwen-turbo"
	APIURL string `yaml:"api_url"` // API 地址
	APIKey string `yaml:"api_key"` // API Key
	Model  string `yaml:"model"`   // 模型名称或接入点 ID
}

// LLMConfig 大模型对话配置。
type LLMConfig struct {
	// Models 多模型优先级列表，按顺序尝试，失败自动切换到下一个。
	// 当此列表非空时，忽略下方的 provider/api_url/api_key/model 字段。
	Models []LLMModelConfig `yaml:"models"`

	// 以下为兼容旧配置的单模型字段（当 Models 为空时使用）
	Provider     string `yaml:"provider"`
	APIURL       string `yaml:"api_url"`
	APIKey       string `yaml:"api_key"`
	Model        string `yaml:"model"`
	SystemPrompt string `yaml:"system_prompt"`
	MaxHistory   int    `yaml:"max_history"`
	MaxTokens    int    `yaml:"max_tokens"`
}

// TTSConfig 语音合成配置。
type TTSConfig struct {
	Engine        string        `yaml:"engine"`
	Fallback      string        `yaml:"fallback"` // 回退引擎，当主引擎失败时使用（如 "piper"、"say"）
	Edge          EdgeConfig    `yaml:"edge"`
	Piper         PiperConfig   `yaml:"piper"`
	Say           SayConfig     `yaml:"say"`
	Tencent       TencentConfig `yaml:"tencent"`
}

// TencentConfig 腾讯云 TTS 配置。
type TencentConfig struct {
	SecretID  string  `yaml:"secret_id"`
	SecretKey string  `yaml:"secret_key"`
	VoiceType int64   `yaml:"voice_type"`
	Region    string  `yaml:"region"`
	Speed     float64 `yaml:"speed"`
}

// EdgeConfig Edge TTS 配置。
type EdgeConfig struct {
	Voice string `yaml:"voice"`
}

// PiperConfig Piper TTS 配置。
type PiperConfig struct {
	ModelPath string `yaml:"model_path"`
}

// SayConfig macOS say TTS 配置。
type SayConfig struct {
	Voice string `yaml:"voice"` // macOS 语音名称，如 "Tingting"（中文），为空使用系统默认
}

// ToolsConfig 工具配置。
type ToolsConfig struct {
	DataDir       string              `yaml:"data_dir"`
	Weather       WeatherConfig       `yaml:"weather"`
	Music         MusicConfig         `yaml:"music"`
	RSS           RSSConfig           `yaml:"rss"`
	Timer         TimerConfig         `yaml:"timer"`
	Volume        VolumeConfig        `yaml:"volume"`
	Translate     TranslateConfig     `yaml:"translate"`
	HomeAssistant HomeAssistantConfig `yaml:"home_assistant"`
	Health        HealthConfig        `yaml:"health"`
	Ezviz         EzvizConfig         `yaml:"ezviz"`
	Learning      LearningConfig      `yaml:"learning"`
	Story         StoryConfig         `yaml:"story"`
}

// LearningConfig 学习工具配置。
type LearningConfig struct {
	Enabled bool            `yaml:"enabled"`
	English EnglishConfig   `yaml:"english"`
	Poetry  PoetryAPIConfig `yaml:"poetry"`
}

// StoryConfig 故事功能配置。
type StoryConfig struct {
	Enabled     bool            `yaml:"enabled"`
	API         StoryAPIConfig  `yaml:"api"`           // 外部 API 配置
	LLMFallback bool            `yaml:"llm_fallback"`  // LLM 兜底开关
	OutputMode  string          `yaml:"output_mode"`   // 输出模式：raw（原文朗读）、summarize（LLM 总结）
}

// StoryAPIConfig 故事 API 配置。
type StoryAPIConfig struct {
	Enabled   bool   `yaml:"enabled"`
	BaseURL   string `yaml:"base_url"`
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}

// EnglishConfig 英语学习配置。
type EnglishConfig struct {
	Enabled bool `yaml:"enabled"`
}

// PoetryAPIConfig 古诗词配置。
type PoetryAPIConfig struct {
	Enabled bool   `yaml:"enabled"`
	APIKey  string `yaml:"api_key"` // 诗词六六六 API Key（可选）
}

// EzvizConfig 萤石开放平台配置。
type EzvizConfig struct {
	Enabled      bool   `yaml:"enabled"`
	AppKey       string `yaml:"app_key"`
	AppSecret    string `yaml:"app_secret"`
	DeviceSerial string `yaml:"device_serial"` // 默认门锁序列号
}

// HealthConfig 健康提醒配置。
type HealthConfig struct {
	Enabled          bool             `yaml:"enabled"`
	WaterInterval    int              `yaml:"water_interval"`    // 默认喝水间隔（分钟）
	ExerciseInterval int              `yaml:"exercise_interval"` // 默认久坐间隔（分钟）
	QuietHours       QuietHoursConfig `yaml:"quiet_hours"`
}

// QuietHoursConfig 静音时段配置。
type QuietHoursConfig struct {
	Start string `yaml:"start"` // 静音开始时间，如 "23:00"
	End   string `yaml:"end"`   // 静音结束时间，如 "07:00"
}

// HomeAssistantConfig Home Assistant 配置。
type HomeAssistantConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Token   string `yaml:"token"`
}

// TranslateConfig 翻译配置。
type TranslateConfig struct {
	Enabled   bool   `yaml:"enabled"`
	SecretID  string `yaml:"secret_id"`
	SecretKey string `yaml:"secret_key"`
	Region    string `yaml:"region"`
}

// TimerConfig 倒计时配置。
type TimerConfig struct {
	MaxConcurrent int `yaml:"max_concurrent"` // 最大同时运行的倒计时数，默认 5
}

// VolumeConfig 音量控制配置。
type VolumeConfig struct {
	Step int `yaml:"step"` // 相对调节步长，默认 10
}

// RSSConfig RSS 订阅功能配置。
type RSSConfig struct {
	Enabled  bool `yaml:"enabled"`
	CacheTTL int  `yaml:"cache_ttl"` // 缓存有效期（分钟），默认 30
}

// MusicConfig 音乐服务配置。
type MusicConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Provider     string `yaml:"provider"`       // netease 或 qq
	APIURL       string `yaml:"api_url"`         // 兼容旧配置
	CacheDir     string `yaml:"cache_dir"`       // 缓存目录，默认 {DataDir}/music_cache
	CacheMaxSize int64  `yaml:"cache_max_size"`  // 缓存最大大小（MB），默认 500，0 表示禁用缓存
	Netease      struct {
		APIURL string `yaml:"api_url"` // 网易云 API 地址
	} `yaml:"netease"`
	QQ struct {
		APIURL string `yaml:"api_url"` // QQ 音乐 API 地址
	} `yaml:"qq"`
}

// WeatherConfig 和风天气配置。
type WeatherConfig struct {
	APIKey  string `yaml:"api_key"`
	APIHost string `yaml:"api_host"`
	// JWT 认证（推荐）
	CredentialID   string `yaml:"credential_id"`
	ProjectID      string `yaml:"project_id"`
	PrivateKeyPath string `yaml:"private_key_path"`
}

// LogConfig 日志配置。
type LogConfig struct {
	Level      string `yaml:"level"`       // 日志级别: debug, info, warn, error
	File       string `yaml:"file"`        // 日志文件路径，为空则只输出到控制台
	MaxSize    int    `yaml:"max_size"`    // 单个日志文件最大大小（MB），默认 64
	MaxBackups int    `yaml:"max_backups"` // 保留的旧日志文件最大数量，默认 3
	MaxAge     int    `yaml:"max_age"`     // 保留旧日志文件的最大天数，默认 7
}

// Load 读取 YAML 配置文件并返回 Config。
// 支持 ${VAR_NAME} 形式的环境变量展开。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
	}

	// 展开环境变量，如 ${PIBUDDY_LLM_API_KEY}
	expanded := os.Expand(string(data), func(key string) string {
		return os.Getenv(key)
	})

	cfg := &Config{}
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
	}

	setDefaults(cfg)
	return cfg, nil
}

// setDefaults 为未设置的配置项填充默认值。
func setDefaults(cfg *Config) {
	if cfg.Audio.SampleRate == 0 {
		cfg.Audio.SampleRate = 16000
	}
	if cfg.Audio.Channels == 0 {
		cfg.Audio.Channels = 1
	}
	if cfg.Audio.FrameSize == 0 {
		cfg.Audio.FrameSize = 512
	}
	if cfg.Wake.Threshold == 0 {
		cfg.Wake.Threshold = 0.5
	}
	if cfg.VAD.Threshold == 0 {
		cfg.VAD.Threshold = 0.5
	}
	if cfg.VAD.MinSilenceMs == 0 {
		cfg.VAD.MinSilenceMs = 1200
	}
	if cfg.ASR.NumThreads == 0 {
		cfg.ASR.NumThreads = 2
	}
	// ASR 多引擎优先级默认值
	if len(cfg.ASR.Priority) == 0 {
		// 兼容旧配置：从 provider + fallback 构建优先级列表
		if cfg.ASR.Provider != "" {
			cfg.ASR.Priority = append(cfg.ASR.Priority, cfg.ASR.Provider)
		}
		if cfg.ASR.Fallback != "" && cfg.ASR.Fallback != cfg.ASR.Provider {
			cfg.ASR.Priority = append(cfg.ASR.Priority, cfg.ASR.Fallback)
		}
		// 确保 sherpa 在列表中
		hasSherpa := false
		for _, p := range cfg.ASR.Priority {
			if p == "sherpa" {
				hasSherpa = true
				break
			}
		}
		if !hasSherpa {
			cfg.ASR.Priority = append(cfg.ASR.Priority, "sherpa")
		}
		// 如果列表仍为空，使用默认值
		if len(cfg.ASR.Priority) == 0 {
			cfg.ASR.Priority = []string{"tencent-flash", "tencent-rt", "sherpa"}
		}
	} else {
		// 确保 priority 列表中有 sherpa 作为兜底
		hasSherpa := false
		for _, p := range cfg.ASR.Priority {
			if p == "sherpa" {
				hasSherpa = true
				break
			}
		}
		if !hasSherpa {
			cfg.ASR.Priority = append(cfg.ASR.Priority, "sherpa")
		}
	}
	if cfg.ASR.Tencent.Region == "" {
		cfg.ASR.Tencent.Region = "ap-guangzhou"
	}
	if cfg.LLM.MaxHistory == 0 {
		cfg.LLM.MaxHistory = 10
	}
	if cfg.LLM.MaxTokens == 0 {
		cfg.LLM.MaxTokens = 500
	}
	if cfg.TTS.Engine == "" {
		cfg.TTS.Engine = "tencent"
	}
	if cfg.TTS.Edge.Voice == "" {
		cfg.TTS.Edge.Voice = "zh-CN-XiaoxiaoNeural"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.MaxSize == 0 {
		cfg.Log.MaxSize = 64 // 默认 64MB
	}
	if cfg.Log.MaxBackups == 0 {
		cfg.Log.MaxBackups = 3 // 默认保留 3 个备份
	}
	if cfg.Log.MaxAge == 0 {
		cfg.Log.MaxAge = 7 // 默认保留 7 天
	}
	if cfg.Dialog.ContinuousTimeout == 0 {
		cfg.Dialog.ContinuousTimeout = 8 // 默认 8 秒
	}
	if cfg.Dialog.ListenDelay == 0 {
		cfg.Dialog.ListenDelay = 500 // 默认 500ms
	}

	if cfg.Voiceprint.Threshold == 0 {
		cfg.Voiceprint.Threshold = 0.6
	}
	if cfg.Voiceprint.NumThreads == 0 {
		cfg.Voiceprint.NumThreads = 1
	}
	if cfg.Voiceprint.BufferSecs == 0 {
		cfg.Voiceprint.BufferSecs = 3.0
	}

	if cfg.Tools.DataDir == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			cfg.Tools.DataDir = home + "/.pibuddy"
		} else {
			cfg.Tools.DataDir = "./.pibuddy-data"
		}
	} else if strings.HasPrefix(cfg.Tools.DataDir, "~/") {
		// Go 不会自动展开 ~，需要手动替换为用户主目录
		home, _ := os.UserHomeDir()
		if home != "" {
			cfg.Tools.DataDir = home + cfg.Tools.DataDir[1:]
		}
	}

	// 音乐缓存默认值
	if cfg.Tools.Music.CacheDir == "" {
		cfg.Tools.Music.CacheDir = cfg.Tools.DataDir + "/music_cache"
	} else if strings.HasPrefix(cfg.Tools.Music.CacheDir, "~/") {
		home, _ := os.UserHomeDir()
		if home != "" {
			cfg.Tools.Music.CacheDir = home + cfg.Tools.Music.CacheDir[1:]
		}
	}
	if cfg.Tools.Music.CacheMaxSize == 0 {
		cfg.Tools.Music.CacheMaxSize = 500 // 默认 500MB
	}

	// 倒计时默认值
	if cfg.Tools.Timer.MaxConcurrent == 0 {
		cfg.Tools.Timer.MaxConcurrent = 5
	}

	// 音量控制默认值
	if cfg.Tools.Volume.Step == 0 {
		cfg.Tools.Volume.Step = 10
	}

	// 故事功能默认值
	if cfg.Tools.Story.API.BaseURL == "" {
		cfg.Tools.Story.API.BaseURL = "https://www.mxnzp.com"
	}
	if cfg.Tools.Story.OutputMode == "" {
		cfg.Tools.Story.OutputMode = "raw" // 默认原文朗读
	}

	// 去除 API Key 两端可能的空白（环境变量展开后常见）
	cfg.LLM.APIKey = strings.TrimSpace(cfg.LLM.APIKey)
	// 多模型配置：trim 所有 API Key
	for i := range cfg.LLM.Models {
		cfg.LLM.Models[i].APIKey = strings.TrimSpace(cfg.LLM.Models[i].APIKey)
	}
	// 兼容旧配置：如果 Models 为空且旧字段有值，构建单元素 Models 列表
	if len(cfg.LLM.Models) == 0 && cfg.LLM.APIURL != "" {
		cfg.LLM.Models = []LLMModelConfig{
			{
				Name:   cfg.LLM.Model,
				APIURL: cfg.LLM.APIURL,
				APIKey: cfg.LLM.APIKey,
				Model:  cfg.LLM.Model,
			},
		}
	}
}
