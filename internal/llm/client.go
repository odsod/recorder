package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/config"
)

var jsonExtractRe = regexp.MustCompile(`\{[\s\S]*\}`)

type Client struct {
	http *http.Client
	cfg  config.LLMConfig
}

func New(httpClient *http.Client, cfg config.LLMConfig) *Client {
	return &Client{http: httpClient, cfg: cfg}
}

func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	payload := map[string]any{
		"model": c.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.3,
		"max_tokens":  4096,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(c.cfg.TimeoutS)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", errors.New("no choices in response")
	}

	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func (c *Client) CompleteJSON(ctx context.Context, system, user string) (map[string]any, error) {
	content, err := c.Complete(ctx, system, user)
	if err != nil {
		return nil, err
	}

	m := jsonExtractRe.FindString(content)
	if m == "" {
		return nil, errors.New("no JSON in response")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(m), &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func (c *Client) CompleteJSONWithRetry(
	ctx context.Context,
	system, user string,
	maxRetries int,
) (map[string]any, error) {
	var lastErr error
	for range 1 + maxRetries {
		result, err := c.CompleteJSON(ctx, system, user)
		if err == nil && result != nil {
			return result, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	return nil, fmt.Errorf("llm failed after %d attempts: %w", 1+maxRetries, lastErr)
}
