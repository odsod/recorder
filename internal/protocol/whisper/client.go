// Package whisper provides a client for OpenAI-compatible audio transcription APIs.
//
// The client sends WAV audio data as a multipart form upload and receives
// transcribed text. It normalizes whitespace in the response as a protocol-level
// concern (Whisper servers sometimes return irregular spacing).
package whisper

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
)

var spaceRe = regexp.MustCompile(`\s+`)

// Config holds connection parameters for the whisper client.
type Config struct {
	// URL is the transcription endpoint (e.g. "http://localhost:8178/v1/audio/transcriptions").
	URL string
	// Timeout is the per-request deadline including upload and server processing.
	Timeout time.Duration
}

// TranscriptionError is returned when the server responds with a non-200 status.
type TranscriptionError struct {
	StatusCode int
	Body       []byte
}

func (e *TranscriptionError) Error() string {
	return fmt.Sprintf("whisper: status %d", e.StatusCode)
}

// TranscribeRequest contains the audio data to transcribe.
type TranscribeRequest struct {
	// WAVData is the raw WAV-encoded audio bytes.
	WAVData []byte
	// Filename is sent as the multipart form filename (e.g. "sys.wav").
	Filename string
}

// TranscribeResponse contains the transcription result.
type TranscribeResponse struct {
	// Text is the transcribed speech with normalized whitespace.
	Text string
}

// Client communicates with an OpenAI-compatible audio transcription endpoint.
type Client struct {
	http *http.Client
	cfg  Config
}

// New creates a Client with the given HTTP client and configuration.
func New(httpClient *http.Client, cfg Config) *Client {
	return &Client{http: httpClient, cfg: cfg}
}

// Transcribe uploads audio and returns the transcribed text.
// Returns a TranscriptionError for non-200 HTTP responses.
func (c *Client) Transcribe(ctx context.Context, req TranscribeRequest) (TranscribeResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", req.Filename)
	if err != nil {
		return TranscribeResponse{}, err
	}
	if _, err := part.Write(req.WAVData); err != nil {
		return TranscribeResponse{}, err
	}

	if err := writer.WriteField("model", "whisper-1"); err != nil {
		return TranscribeResponse{}, err
	}
	if err := writer.WriteField("response_format", "json"); err != nil {
		return TranscribeResponse{}, err
	}
	if err := writer.Close(); err != nil {
		return TranscribeResponse{}, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.cfg.URL, &body)
	if err != nil {
		return TranscribeResponse{}, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return TranscribeResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return TranscribeResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return TranscribeResponse{}, &TranscriptionError{StatusCode: resp.StatusCode, Body: respBody}
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return TranscribeResponse{}, err
	}

	text := strings.TrimSpace(result.Text)
	text = spaceRe.ReplaceAllString(text, " ")
	return TranscribeResponse{Text: text}, nil
}
