package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetDefaults_EmptyConfig(t *testing.T) {
	cfg := &Config{}
	setDefaults(cfg)

	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Audio.SampleRate", cfg.Audio.SampleRate, 16000},
		{"Audio.Channels", cfg.Audio.Channels, 1},
		{"Audio.FrameSize", cfg.Audio.FrameSize, 512},
		{"Wake.Threshold", cfg.Wake.Threshold, float32(0.5)},
		{"VAD.Threshold", cfg.VAD.Threshold, float32(0.5)},
		{"VAD.MinSilenceMs", cfg.VAD.MinSilenceMs, 500},
		{"ASR.NumThreads", cfg.ASR.NumThreads, 2},
		{"LLM.MaxHistory", cfg.LLM.MaxHistory, 10},
		{"LLM.MaxTokens", cfg.LLM.MaxTokens, 500},
		{"TTS.Engine", cfg.TTS.Engine, "edge"},
		{"TTS.Edge.Voice", cfg.TTS.Edge.Voice, "zh-CN-XiaoxiaoNeural"},
		{"Log.Level", cfg.Log.Level, "info"},
	}

	for _, c := range checks {
		switch want := c.want.(type) {
		case int:
			if c.got.(int) != want {
				t.Errorf("%s: got %v, want %v", c.name, c.got, want)
			}
		case float32:
			if c.got.(float32) != want {
				t.Errorf("%s: got %v, want %v", c.name, c.got, want)
			}
		case string:
			if c.got.(string) != want {
				t.Errorf("%s: got %v, want %v", c.name, c.got, want)
			}
		}
	}
}

func TestSetDefaults_DoesNotOverride(t *testing.T) {
	cfg := &Config{
		Audio: AudioConfig{SampleRate: 44100, Channels: 2, FrameSize: 1024},
		LLM:   LLMConfig{MaxHistory: 20, MaxTokens: 1000},
		TTS:   TTSConfig{Engine: "piper", Edge: EdgeConfig{Voice: "custom-voice"}},
		Log:   LogConfig{Level: "debug"},
	}
	setDefaults(cfg)

	if cfg.Audio.SampleRate != 44100 {
		t.Errorf("SampleRate should not be overridden: got %d", cfg.Audio.SampleRate)
	}
	if cfg.Audio.Channels != 2 {
		t.Errorf("Channels should not be overridden: got %d", cfg.Audio.Channels)
	}
	if cfg.Audio.FrameSize != 1024 {
		t.Errorf("FrameSize should not be overridden: got %d", cfg.Audio.FrameSize)
	}
	if cfg.LLM.MaxHistory != 20 {
		t.Errorf("MaxHistory should not be overridden: got %d", cfg.LLM.MaxHistory)
	}
	if cfg.LLM.MaxTokens != 1000 {
		t.Errorf("MaxTokens should not be overridden: got %d", cfg.LLM.MaxTokens)
	}
	if cfg.TTS.Engine != "piper" {
		t.Errorf("TTS.Engine should not be overridden: got %s", cfg.TTS.Engine)
	}
	if cfg.TTS.Edge.Voice != "custom-voice" {
		t.Errorf("TTS.Edge.Voice should not be overridden: got %s", cfg.TTS.Edge.Voice)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level should not be overridden: got %s", cfg.Log.Level)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	yamlContent := `
audio:
  sample_rate: 48000
  channels: 2
  frame_size: 256
llm:
  provider: openai
  api_url: https://api.example.com
  api_key: test-key
  model: gpt-4
  system_prompt: "you are a bot"
  max_history: 5
  max_tokens: 200
tts:
  engine: piper
  piper:
    model_path: /path/to/model
log:
  level: debug
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Audio.SampleRate != 48000 {
		t.Errorf("Audio.SampleRate: got %d, want 48000", cfg.Audio.SampleRate)
	}
	if cfg.Audio.Channels != 2 {
		t.Errorf("Audio.Channels: got %d, want 2", cfg.Audio.Channels)
	}
	if cfg.LLM.APIKey != "test-key" {
		t.Errorf("LLM.APIKey: got %q, want %q", cfg.LLM.APIKey, "test-key")
	}
	if cfg.TTS.Engine != "piper" {
		t.Errorf("TTS.Engine: got %q, want %q", cfg.TTS.Engine, "piper")
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level: got %q, want %q", cfg.Log.Level, "debug")
	}
	// Defaults should be applied for unset fields
	if cfg.Wake.Threshold != 0.5 {
		t.Errorf("Wake.Threshold should default to 0.5, got %f", cfg.Wake.Threshold)
	}
}

func TestLoad_EnvVarExpansion(t *testing.T) {
	t.Setenv("TEST_API_KEY", "secret-from-env")

	yamlContent := `
llm:
  api_key: "${TEST_API_KEY}"
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.LLM.APIKey != "secret-from-env" {
		t.Errorf("expected env var expansion, got %q", cfg.LLM.APIKey)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestSetDefaults_TrimsAPIKey(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{APIKey: "  key-with-spaces  "},
	}
	setDefaults(cfg)
	if cfg.LLM.APIKey != "key-with-spaces" {
		t.Errorf("expected trimmed API key, got %q", cfg.LLM.APIKey)
	}
}
