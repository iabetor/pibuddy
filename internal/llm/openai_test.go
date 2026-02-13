package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChatStream_NormalSSE(t *testing.T) {
	sseBody := `data: {"choices":[{"delta":{"content":"Hello"}}]}

data: {"choices":[{"delta":{"content":" world"}}]}

data: [DONE]
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseBody)
	}))
	defer server.Close()

	provider := NewOpenAIProvider(server.URL, "test-key", "test-model")
	messages := []Message{{Role: "user", Content: "hi"}}

	ch, err := provider.ChatStream(context.Background(), messages)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	var chunks []string
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "Hello" {
		t.Errorf("chunk[0] = %q, want %q", chunks[0], "Hello")
	}
	if chunks[1] != " world" {
		t.Errorf("chunk[1] = %q, want %q", chunks[1], " world")
	}
}

func TestChatStream_EmptyDeltaSkipped(t *testing.T) {
	sseBody := `data: {"choices":[{"delta":{"content":""}}]}

data: {"choices":[{"delta":{"content":"text"}}]}

data: {"choices":[{"delta":{}}]}

data: [DONE]
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseBody)
	}))
	defer server.Close()

	provider := NewOpenAIProvider(server.URL, "key", "model")
	ch, err := provider.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	var chunks []string
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk (empty deltas skipped), got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "text" {
		t.Errorf("expected %q, got %q", "text", chunks[0])
	}
}

func TestChatStream_Non200StatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":"rate limited"}`)
	}))
	defer server.Close()

	provider := NewOpenAIProvider(server.URL, "key", "model")
	_, err := provider.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should contain status code 429: %v", err)
	}
}

func TestChatStream_ContextCancel(t *testing.T) {
	// Server that streams slowly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected ResponseWriter to be a Flusher")
		}

		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"first\"}}]}\n\n")
		flusher.Flush()

		// Wait long enough that context should be cancelled
		time.Sleep(2 * time.Second)

		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"second\"}}]}\n\n")
		flusher.Flush()

		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	provider := NewOpenAIProvider(server.URL, "key", "model")
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := provider.ChatStream(ctx, []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	// Read first chunk
	first, ok := <-ch
	if !ok {
		t.Fatal("expected at least one chunk before cancel")
	}
	if first != "first" {
		t.Errorf("expected %q, got %q", "first", first)
	}

	// Cancel context
	cancel()

	// Drain remaining — channel should close soon
	count := 0
	for range ch {
		count++
	}
	// The goroutine should exit quickly after cancel; we may get 0 or 1 extra chunks
	// The important thing is that the channel closes and we don't hang.
}

func TestChatStream_RequestBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := NewOpenAIProvider(server.URL, "key", "model")
	ch, err := provider.ChatStream(context.Background(), []Message{
		{Role: "system", Content: "you are a bot"},
		{Role: "user", Content: "hi"},
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	// Drain
	for range ch {
	}
}
