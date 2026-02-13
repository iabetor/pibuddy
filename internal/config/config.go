package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 是 PiBuddy 的顶层配置结构。
type Config struct {
	Audio AudioConfig `yaml:"audio"`
	Wake  WakeConfig  `yaml:"wake"`
	VAD   VADConfig   `yaml:"vad"`
	ASR   ASRConfig   `yaml:"asr"`
	LLM   LLMConfig   `yaml:"llm"`
	TTS   TTSConfig   `yaml:"tts"`
	Log   LogConfig   `yaml:"log"`
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
	ModelPath  string `yaml:"model_path"`
	NumThreads int    `yaml:"num_threads"`
}

// LLMConfig 大模型对话配置。
type LLMConfig struct {
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
	Engine  string        `yaml:"engine"`
	Edge    EdgeConfig    `yaml:"edge"`
	Piper   PiperConfig   `yaml:"piper"`
	Tencent TencentConfig `yaml:"tencent"`
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

// LogConfig 日志配置。
type LogConfig struct {
	Level string `yaml:"level"`
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

	// 去除 API Key 两端可能的空白（环境变量展开后常见）
	cfg.LLM.APIKey = strings.TrimSpace(cfg.LLM.APIKey)
}
