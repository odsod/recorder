package recorder

import "time"

type AudioChunk struct {
	SysWAV    []byte
	MicWAV    []byte
	StartTime time.Time
	EndTime   time.Time
}
