package transcribe

import (
	"context"
	"strings"

	"github.com/odsod/recorder/internal/protocol/llm"
)

var hallucinationPrefixes = []string{
	"no meaningful", "no speech", "nothing meaningful",
	"the input", "[empty", "i cannot", "i can't",
	"this appears", "this input", "the text",
	"there is no", "there's no", "empty",
}

// ChatCompleter sends chat completion requests to an LLM backend.
type ChatCompleter interface {
	Complete(ctx context.Context, req llm.CompleteRequest) (llm.CompleteResponse, error)
}

// Cleaner post-processes raw ASR text via LLM cleanup.
type Cleaner struct {
	chat   ChatCompleter
	prompt string
}

// NewCleaner creates a Cleaner backed by chat.
func NewCleaner(chat ChatCompleter, prompt string) *Cleaner {
	return &Cleaner{chat: chat, prompt: prompt}
}

// Cleanup removes fillers, fixes grammar, and filters ASR hallucinations.
// When participants are provided, their names are included in the system prompt
// to help the LLM correct ASR misspellings of participant names.
func (c *Cleaner) Cleanup(ctx context.Context, text string, participants []string) (string, error) {
	systemPrompt := c.prompt
	if len(participants) > 0 {
		systemPrompt += "\n\n## Meeting participants\n\nThe following people are in this meeting: " +
			strings.Join(participants, ", ") +
			". Use these exact spellings when correcting names in the transcript."
	}
	resp, err := c.chat.Complete(ctx, llm.CompleteRequest{
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		},
	})
	if err != nil {
		return "", err
	}
	if resp.Content == "" {
		return "", nil
	}

	lower := strings.ToLower(resp.Content)
	for _, prefix := range hallucinationPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return "", nil
		}
	}

	return resp.Content, nil
}
