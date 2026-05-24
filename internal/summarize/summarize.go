package summarize

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/odsod/recorder/internal/protocol/llm"
	"github.com/odsod/recorder/internal/segment"
	"github.com/odsod/recorder/internal/transcript"
)

const (
	chunkChars = 35000
	maxRetries = 2
)

var jsonExtractRe = regexp.MustCompile(`\{[\s\S]*\}`)

type ChatCompleter interface {
	Complete(ctx context.Context, req llm.CompleteRequest) (llm.CompleteResponse, error)
}

type Summarizer struct {
	chat ChatCompleter
}

func NewSummarizer(chat ChatCompleter) *Summarizer {
	return &Summarizer{chat: chat}
}

func (s *Summarizer) SummarizeSegment(
	ctx context.Context,
	seg segment.Segment,
	date string,
) (title, summary string, skip bool, err error) {
	_ = date
	transcriptText := segment.FormatTranscript(seg)
	if strings.TrimSpace(transcriptText) == "" {
		return "", "", true, nil
	}

	var response map[string]any
	if len(transcriptText) <= chunkChars {
		response, err = s.completeJSON(ctx, summarizePrompt, transcriptText)
	} else {
		response, err = s.summarizeChunked(ctx, transcriptText)
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
	sum, _ := response["summary"].(string)
	if t == "" {
		t = "Untitled"
	}
	return t, sum, false, nil
}

func (s *Summarizer) summarizeChunked(ctx context.Context, transcriptText string) (map[string]any, error) {
	chunks := splitIntoChunks(transcriptText, chunkChars)
	var chunkSummaries []map[string]any

	for _, chunk := range chunks {
		result, err := s.completeJSON(ctx, summarizePrompt, chunk)
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
	for _, summary := range chunkSummaries {
		data, _ := json.MarshalIndent(summary, "", "  ")
		parts = append(parts, string(data))
	}
	combinedInput := strings.Join(parts, "\n\n---\n\n")
	return s.completeJSON(ctx, combinePrompt, combinedInput)
}

func (s *Summarizer) completeJSON(ctx context.Context, system, user string) (map[string]any, error) {
	var lastErr error
	for range 1 + maxRetries {
		resp, err := s.chat.Complete(ctx, llm.CompleteRequest{
			Messages: []llm.Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
		})
		if err != nil {
			lastErr = err
			continue
		}
		m := jsonExtractRe.FindString(resp.Content)
		if m == "" {
			lastErr = errors.New("no JSON in response")
			continue
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(m), &parsed); err != nil {
			lastErr = err
			continue
		}
		return parsed, nil
	}
	return nil, fmt.Errorf("llm failed after %d attempts: %w", 1+maxRetries, lastErr)
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

var timeRe = regexp.MustCompile(`^\[(\d{2}):(\d{2})\]`)

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

func ExtractParticipants(seg segment.Segment) []string {
	names := make(map[string]struct{})
	for _, e := range seg.Events {
		if e.Type != transcript.Participants {
			continue
		}
		for _, name := range e.People {
			names[name] = struct{}{}
		}
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	return sorted
}
