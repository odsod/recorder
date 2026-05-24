package capture

import (
	"context"

	"github.com/odsod/recorder/internal/audio/frame"
)

// Source abstracts dual-channel audio capture (system + microphone).
type Source interface {
	Start(ctx context.Context) (<-chan frame.Dual, error)
	Stop() error
	MonitorSource() string
	MicSource() string
}
