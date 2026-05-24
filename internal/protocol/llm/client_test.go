package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/odsod/recorder/internal/protocol/llm"
)

func TestComplete_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}

		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req["model"] != "test-model" {
			t.Errorf("expected model test-model, got %v", req["model"])
		}

		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "  Hello world  "}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := llm.New(srv.Client(), llm.Config{
		URL:         srv.URL,
		Model:       "test-model",
		Timeout:     5 * time.Second,
		Temperature: 0.3,
		MaxTokens:   100,
	})

	resp, err := client.Complete(context.Background(), llm.CompleteRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hi"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", resp.Content)
	}
}

func TestComplete_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"choices": []any{}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := llm.New(srv.Client(), llm.Config{
		URL:     srv.URL,
		Model:   "m",
		Timeout: 5 * time.Second,
	})

	_, err := client.Complete(context.Background(), llm.CompleteRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestComplete_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limited"))
	}))
	defer srv.Close()

	client := llm.New(srv.Client(), llm.Config{
		URL:     srv.URL,
		Model:   "m",
		Timeout: 5 * time.Second,
	})

	_, err := client.Complete(context.Background(), llm.CompleteRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
	ce := &llm.CompletionError{}
	ok := errors.As(err, &ce)
	if !ok {
		t.Fatalf("expected CompletionError, got %T: %v", err, err)
	}
	if ce.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", ce.StatusCode)
	}
	if string(ce.Body) != "rate limited" {
		t.Errorf("expected body 'rate limited', got %q", string(ce.Body))
	}
}

func TestComplete_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	client := llm.New(srv.Client(), llm.Config{
		URL:     srv.URL,
		Model:   "m",
		Timeout: 5 * time.Second,
	})

	_, err := client.Complete(context.Background(), llm.CompleteRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestComplete_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))
	defer srv.Close()

	client := llm.New(srv.Client(), llm.Config{
		URL:     srv.URL,
		Model:   "m",
		Timeout: 10 * time.Second,
	})

	_, err := client.Complete(ctx, llm.CompleteRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestComplete_RequestPayload(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "ok"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := llm.New(srv.Client(), llm.Config{
		URL:         srv.URL,
		Model:       "gpt-4",
		Timeout:     5 * time.Second,
		Temperature: 0.7,
		MaxTokens:   2048,
	})

	_, _ = client.Complete(context.Background(), llm.CompleteRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "usr"},
		},
	})

	if received["model"] != "gpt-4" {
		t.Errorf("model: got %v", received["model"])
	}
	if received["temperature"] != 0.7 {
		t.Errorf("temperature: got %v", received["temperature"])
	}
	if received["max_tokens"] != float64(2048) {
		t.Errorf("max_tokens: got %v", received["max_tokens"])
	}
	msgs := received["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	m0 := msgs[0].(map[string]any)
	if m0["role"] != "system" || m0["content"] != "sys" {
		t.Errorf("message 0: %v", m0)
	}
}
