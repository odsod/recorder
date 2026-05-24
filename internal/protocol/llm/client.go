// Package llm provides a client for OpenAI-compatible chat completion APIs.
//
// The client sends a list of messages and receives a single text completion.
// It handles request serialization, HTTP transport, and response parsing.
// Application-level concerns like retries, JSON extraction from responses,
// and prompt engineering belong in the caller.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message is a single entry in the chat conversation.
type Message struct {
	// Role is typically "system", "user", or "assistant".
	Role string
	// Content is the text body of the message.
	Content string
}

// Config holds connection and model parameters for the client.
type Config struct {
	// URL is the chat completions endpoint (e.g. "http://localhost:8179/v1/chat/completions").
	URL string
	// Model is the model identifier sent in each request.
	Model string
	// Timeout is the per-request deadline including network and server processing.
	Timeout time.Duration
	// Temperature controls randomness in the response (0.0–2.0).
	Temperature float64
	// MaxTokens caps the length of the generated completion.
	MaxTokens int
}

// CompletionError is returned when the server responds with a non-200 status.
type CompletionError struct {
	StatusCode int
	Body       []byte
}

func (e *CompletionError) Error() string {
	return fmt.Sprintf("chat completion: status %d", e.StatusCode)
}

// CompleteRequest contains the messages to send for completion.
type CompleteRequest struct {
	Messages []Message
}

// CompleteResponse contains the generated text from the model.
type CompleteResponse struct {
	// Content is the trimmed text content of the first choice.
	Content string
}

// Client communicates with an OpenAI-compatible chat completions endpoint.
type Client struct {
	http *http.Client
	cfg  Config
}

// New creates a Client with the given HTTP client and configuration.
func New(httpClient *http.Client, cfg Config) *Client {
	return &Client{http: httpClient, cfg: cfg}
}

// Complete sends a chat completion request and returns the model's response.
// Returns a CompletionError for non-200 HTTP responses.
func (c *Client) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	msgs := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = map[string]string{"role": m.Role, "content": m.Content}
	}

	payload := map[string]any{
		"model":       c.cfg.Model,
		"messages":    msgs,
		"temperature": c.cfg.Temperature,
		"max_tokens":  c.cfg.MaxTokens,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return CompleteResponse{}, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return CompleteResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return CompleteResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompleteResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return CompleteResponse{}, &CompletionError{StatusCode: resp.StatusCode, Body: respBody}
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return CompleteResponse{}, err
	}
	if len(result.Choices) == 0 {
		return CompleteResponse{}, errors.New("chat completion: no choices in response")
	}

	return CompleteResponse{
		Content: strings.TrimSpace(result.Choices[0].Message.Content),
	}, nil
}
