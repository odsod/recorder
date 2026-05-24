package recorder

import "time"

// AudioChunk holds WAV-encoded audio for both channels with wall-clock timestamps.
type AudioChunk struct {
	SysWAV    []byte
	MicWAV    []byte
	StartTime time.Time
	EndTime   time.Time
}
