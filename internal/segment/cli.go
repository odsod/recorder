package segment

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/odsod/recorder/internal/transcript"
)

// PrintBoundaries prints detected segment boundaries to stdout.
func PrintBoundaries(events []transcript.Event) {
	boundaries := DetectBoundaries(events, time.Now())
	for _, b := range boundaries {
		fmt.Printf("[%s] %s\n", b.Time.Format("15:04:05"), b.Reason)
	}
}

// RunBatch lists detected segments and optionally writes summary files.
// handler may be nil when write is false.
func RunBatch(ctx context.Context, events []transcript.Event, write bool, handler Handler) error {
	boundaries := DetectBoundaries(events, time.Now())
	segments := SplitAtBoundaries(events, boundaries)

	for _, seg := range segments {
		speechCount := 0
		for _, e := range seg.Events {
			if e.IsSpeech() {
				speechCount++
			}
		}
		fmt.Printf("segment %s: %s–%s (%d speech events)\n",
			seg.ID, seg.Start.Format("15:04"), seg.End.Format("15:04"), speechCount)
	}

	if !write {
		return nil
	}

	date := time.Now().Format("2006-01-02")
	for _, seg := range segments {
		if err := ctx.Err(); err != nil {
			return err
		}

		title, summary, skip, err := handler.Summarize(ctx, seg, date)
		if err != nil {
			slog.ErrorContext(ctx, "summarize failed",
				"segmentId", seg.ID,
				"err", err,
			)
			continue
		}
		if skip {
			fmt.Printf("  %s: skipped\n", seg.ID)
			continue
		}
		filename, err := handler.WriteSegment(title, summary, seg, date)
		if err != nil {
			slog.ErrorContext(ctx, "write segment failed",
				"segmentId", seg.ID,
				"err", err,
			)
			continue
		}
		fmt.Printf("  %s: wrote %s\n", seg.ID, filename)
	}
	return nil
}
