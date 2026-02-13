package pipeline

import "testing"

func TestExtractSentence_ChinesePunctuation(t *testing.T) {
	tests := []struct {
		input     string
		sentence  string
		remainder string
	}{
		{"你好。世界", "你好。", "世界"},
		{"你好！世界", "你好！", "世界"},
		{"你好？世界", "你好？", "世界"},
		{"你好；世界", "你好；", "世界"},
	}

	for _, tt := range tests {
		sentence, remainder, found := extractSentence(tt.input)
		if !found {
			t.Errorf("extractSentence(%q): expected found=true", tt.input)
			continue
		}
		if sentence != tt.sentence {
			t.Errorf("extractSentence(%q): sentence = %q, want %q", tt.input, sentence, tt.sentence)
		}
		if remainder != tt.remainder {
			t.Errorf("extractSentence(%q): remainder = %q, want %q", tt.input, remainder, tt.remainder)
		}
	}
}

func TestExtractSentence_EnglishPunctuation(t *testing.T) {
	tests := []struct {
		input     string
		sentence  string
		remainder string
	}{
		{"Hello. World", "Hello.", " World"},
		{"Hello! World", "Hello!", " World"},
		{"Hello? World", "Hello?", " World"},
	}

	for _, tt := range tests {
		sentence, remainder, found := extractSentence(tt.input)
		if !found {
			t.Errorf("extractSentence(%q): expected found=true", tt.input)
			continue
		}
		if sentence != tt.sentence {
			t.Errorf("extractSentence(%q): sentence = %q, want %q", tt.input, sentence, tt.sentence)
		}
		if remainder != tt.remainder {
			t.Errorf("extractSentence(%q): remainder = %q, want %q", tt.input, remainder, tt.remainder)
		}
	}
}

func TestExtractSentence_Newline(t *testing.T) {
	sentence, remainder, found := extractSentence("line1\nline2")
	if !found {
		t.Fatal("expected found=true for newline")
	}
	if sentence != "line1\n" {
		t.Errorf("sentence = %q, want %q", sentence, "line1\n")
	}
	if remainder != "line2" {
		t.Errorf("remainder = %q, want %q", remainder, "line2")
	}
}

func TestExtractSentence_OnlyFirstSentence(t *testing.T) {
	sentence, remainder, found := extractSentence("First. Second. Third.")
	if !found {
		t.Fatal("expected found=true")
	}
	if sentence != "First." {
		t.Errorf("sentence = %q, want %q", sentence, "First.")
	}
	if remainder != " Second. Third." {
		t.Errorf("remainder = %q, want %q", remainder, " Second. Third.")
	}
}

func TestExtractSentence_NoPunctuation(t *testing.T) {
	_, remainder, found := extractSentence("no sentence ending here")
	if found {
		t.Error("expected found=false for text without sentence enders")
	}
	if remainder != "no sentence ending here" {
		t.Errorf("remainder = %q, want original text", remainder)
	}
}

func TestExtractSentence_Empty(t *testing.T) {
	_, remainder, found := extractSentence("")
	if found {
		t.Error("expected found=false for empty string")
	}
	if remainder != "" {
		t.Errorf("remainder = %q, want empty", remainder)
	}
}

func TestExtractSentence_PunctuationOnly(t *testing.T) {
	sentence, remainder, found := extractSentence("。")
	if !found {
		t.Fatal("expected found=true")
	}
	if sentence != "。" {
		t.Errorf("sentence = %q, want %q", sentence, "。")
	}
	if remainder != "" {
		t.Errorf("remainder = %q, want empty", remainder)
	}
}
