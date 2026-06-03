package recorder

import (
	"context"
	"log/slog"

	"github.com/odsod/recorder/internal/transcript"
)

func (r *Recorder) appendEvent(ctx context.Context, e transcript.Event) {
	r.transcript.AppendEvent(e)
	slog.InfoContext(ctx, "transcript event",
		"line", e.String(),
	)
}

func setsEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}
