package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/config"
)

var spaceRe = regexp.MustCompile(`\s+`)

type WhisperClient struct {
	http *http.Client
	cfg  config.WhisperConfig
}

func NewWhisperClient(httpClient *http.Client, cfg config.WhisperConfig) *WhisperClient {
	return &WhisperClient{http: httpClient, cfg: cfg}
}

func (c *WhisperClient) Transcribe(ctx context.Context, wavData []byte, filename string) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(wavData); err != nil {
		return "", err
	}

	if err := writer.WriteField("model", "whisper-1"); err != nil {
		return "", err
	}
	if err := writer.WriteField("response_format", "json"); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(c.cfg.TimeoutS)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.cfg.URL, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("whisper server returned %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	text := strings.TrimSpace(result.Text)
	text = spaceRe.ReplaceAllString(text, " ")
	return text, nil
}
