package llm

import (
	"strings"
	"testing"
)

func TestContextManager_AddAndMessages(t *testing.T) {
	cm := NewContextManager("you are a bot", 5)

	cm.Add("user", "hello")
	cm.Add("assistant", "hi")

	msgs := cm.Messages()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (system + 2), got %d", len(msgs))
	}
	if msgs[0].Role != "system" || !strings.HasPrefix(msgs[0].Content, "you are a bot") {
		t.Errorf("first message should start with system prompt, got %+v", msgs[0])
	}
	// system prompt 应包含动态注入的当前时间
	if !strings.Contains(msgs[0].Content, "当前时间:") {
		t.Errorf("system prompt should contain current time, got %q", msgs[0].Content)
	}
	if msgs[1].Role != "user" || msgs[1].Content != "hello" {
		t.Errorf("second message mismatch: %+v", msgs[1])
	}
	if msgs[2].Role != "assistant" || msgs[2].Content != "hi" {
		t.Errorf("third message mismatch: %+v", msgs[2])
	}
}

func TestContextManager_OrderPreserved(t *testing.T) {
	cm := NewContextManager("sys", 10)

	for i := 0; i < 6; i++ {
		cm.Add("user", string(rune('a'+i)))
	}

	msgs := cm.Messages()
	// system + 6 messages
	if len(msgs) != 7 {
		t.Fatalf("expected 7 messages, got %d", len(msgs))
	}
	for i := 1; i < len(msgs); i++ {
		expected := string(rune('a' + i - 1))
		if msgs[i].Content != expected {
			t.Errorf("index %d: expected %q, got %q", i, expected, msgs[i].Content)
		}
	}
}

func TestContextManager_SlidingWindow(t *testing.T) {
	cm := NewContextManager("sys", 2) // max 2 rounds = 4 messages

	// Add 6 messages (3 rounds), should keep only the last 4
	cm.Add("user", "q1")
	cm.Add("assistant", "a1")
	cm.Add("user", "q2")
	cm.Add("assistant", "a2")
	cm.Add("user", "q3")
	cm.Add("assistant", "a3")

	msgs := cm.Messages()
	// system + 4 most recent
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages (system + 4), got %d", len(msgs))
	}

	if msgs[1].Content != "q2" {
		t.Errorf("expected first kept message to be 'q2', got %q", msgs[1].Content)
	}
	if msgs[4].Content != "a3" {
		t.Errorf("expected last message to be 'a3', got %q", msgs[4].Content)
	}
}

func TestContextManager_MessagesAlwaysStartsWithSystem(t *testing.T) {
	cm := NewContextManager("system prompt", 5)
	msgs := cm.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (system only), got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("expected system role, got %q", msgs[0].Role)
	}
	if !strings.HasPrefix(msgs[0].Content, "system prompt") {
		t.Errorf("expected system prompt prefix, got %q", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "当前时间:") {
		t.Errorf("system prompt should contain current time, got %q", msgs[0].Content)
	}
}

func TestContextManager_Clear(t *testing.T) {
	cm := NewContextManager("sys", 5)
	cm.Add("user", "hello")
	cm.Add("assistant", "hi")
	cm.Clear()

	msgs := cm.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message after clear, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("expected system role, got %q", msgs[0].Role)
	}
}
