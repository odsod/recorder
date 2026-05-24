// Package conference defines the common interface for video conferencing
// platform integrations. Each platform (Google Meet, Microsoft Teams)
// implements Provider to extract participant and speaking state from
// its specific DOM structure via CDP Runtime.evaluate.
package conference

// ParticipantSnapshot captures a participant tile during the discovery phase.
// The detector diffs Classes between consecutive snapshots to identify
// which CSS class toggles indicate speaking state.
type ParticipantSnapshot struct {
	Name    string
	Classes []string
}

// Participant is the poll-phase result after the speaking class is known.
type Participant struct {
	Name     string
	Speaking bool
}

// Provider encapsulates platform-specific DOM extraction logic.
// Implementations are stateless — all state lives in the detector.
type Provider interface {
	// Name returns the platform identifier for logging.
	Name() string

	// MatchesURL reports whether a browser tab URL belongs to this platform.
	MatchesURL(url string) bool

	// SnapshotExpression returns JavaScript that captures all participant tiles
	// with their full CSS class sets (discovery phase).
	SnapshotExpression() string

	// ParseSnapshot deserializes the JSON result of SnapshotExpression.
	ParseSnapshot(jsonValue string) ([]ParticipantSnapshot, error)

	// PollExpression returns JavaScript that checks speaking state using the
	// discovered CSS class (cached phase). Returns error if class is invalid.
	PollExpression(speakingClass string) (string, error)

	// ParsePoll deserializes the JSON result of PollExpression.
	ParsePoll(jsonValue string) ([]Participant, error)
}
