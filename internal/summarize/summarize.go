package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/odsod/recorder/internal/config"
	"github.com/odsod/recorder/internal/httpclient"
	"github.com/odsod/recorder/internal/segment"
)

const (
	chunkChars = 35000
	maxRetries = 2
)

func SummarizeSegment(
	ctx context.Context,
	seg segment.Segment,
	cfg config.LLMConfig,
	date string,
) (title, summary string, skip bool, err error) {
	transcriptText := segment.FormatTranscript(seg)
	if strings.TrimSpace(transcriptText) == "" {
		return "", "", true, nil
	}

	var response map[string]any
	if len(transcriptText) <= chunkChars {
		response, err = callLLM(ctx, summarizePrompt, transcriptText, cfg)
	} else {
		response, err = summarizeChunked(ctx, transcriptText, cfg)
	}

	if err != nil {
		return "", "", false, err
	}
	if response == nil {
		return "", "", true, nil
	}
	if _, ok := response["skip"]; ok {
		return "", "", true, nil
	}

	t, _ := response["title"].(string)
	s, _ := response["summary"].(string)
	if t == "" {
		t = "Untitled"
	}
	return t, s, false, nil
}

func summarizeChunked(ctx context.Context, transcript string, cfg config.LLMConfig) (map[string]any, error) {
	chunks := splitIntoChunks(transcript, chunkChars)
	var chunkSummaries []map[string]any

	for _, chunk := range chunks {
		result, err := callLLM(ctx, summarizePrompt, chunk, cfg)
		if err != nil {
			continue
		}
		if result != nil {
			if _, skip := result["skip"]; !skip {
				chunkSummaries = append(chunkSummaries, result)
			}
		}
	}

	if len(chunkSummaries) == 0 {
		return nil, nil
	}
	if len(chunkSummaries) == 1 {
		return chunkSummaries[0], nil
	}

	var parts []string
	for _, s := range chunkSummaries {
		data, _ := json.MarshalIndent(s, "", "  ")
		parts = append(parts, string(data))
	}
	combinedInput := strings.Join(parts, "\n\n---\n\n")
	return callLLM(ctx, combinePrompt, combinedInput, cfg)
}

func splitIntoChunks(text string, maxChars int) []string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil
	}

	var chunks []string
	start := 0

	for start < len(lines) {
		end := start
		total := 0
		for end < len(lines) && total+len(lines[end])+1 <= maxChars {
			total += len(lines[end]) + 1
			end++
		}

		if end >= len(lines) {
			chunks = append(chunks, strings.Join(lines[start:], "\n"))
			break
		}

		splitAt := findBestSplit(lines, start, end)
		chunks = append(chunks, strings.Join(lines[start:splitAt], "\n"))
		start = splitAt
	}
	return chunks
}

var (
	timeRe        = regexp.MustCompile(`^\[(\d{2}):(\d{2})\]`)
	jsonExtractRe = regexp.MustCompile(`\{[\s\S]*\}`)
)

func findBestSplit(lines []string, start, end int) int {
	searchStart := start + (end-start)/2
	bestGap := 0
	bestIdx := end

	for i := searchStart; i < end-1; i++ {
		m1 := timeRe.FindStringSubmatch(lines[i])
		m2 := timeRe.FindStringSubmatch(lines[i+1])
		if m1 != nil && m2 != nil {
			h1, _ := strconv.Atoi(m1[1])
			min1, _ := strconv.Atoi(m1[2])
			h2, _ := strconv.Atoi(m2[1])
			min2, _ := strconv.Atoi(m2[2])
			gap := (h2*60 + min2) - (h1*60 + min1)
			if gap > bestGap {
				bestGap = gap
				bestIdx = i + 1
			}
		}
	}
	return bestIdx
}

func callLLM(ctx context.Context, system, user string, cfg config.LLMConfig) (map[string]any, error) {
	var lastErr error
	for range 1 + maxRetries {
		result, err := doLLMCall(ctx, system, user, cfg)
		if err == nil && result != nil {
			return result, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	return nil, fmt.Errorf("llm failed after %d attempts: %w", 1+maxRetries, lastErr)
}

func doLLMCall(ctx context.Context, system, user string, cfg config.LLMConfig) (map[string]any, error) {
	payload := map[string]any{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.3,
		"max_tokens":  4096,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutS)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", cfg.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.Shared.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if len(result.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	content := result.Choices[0].Message.Content
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

// ExtractParticipants extracts unique participant names from ppl events.
func ExtractParticipants(seg segment.Segment) []string {
	names := make(map[string]struct{})
	pplRe := regexp.MustCompile(`\s*\([^)]*\)`)
	for _, e := range seg.Events {
		if e.Tag != "ppl" {
			continue
		}
		text := pplRe.ReplaceAllString(e.Text, "")
		text = strings.TrimSuffix(text, " joined")
		for name := range strings.SplitSeq(text, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				names[name] = struct{}{}
			}
		}
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	return sorted
}
