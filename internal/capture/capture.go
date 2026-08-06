package capture

import (
	"context"

	"github.com/odsod/recorder/internal/audio/frame"
	"github.com/odsod/recorder/internal/protocol/parec"
)

// Source abstracts dual-channel audio capture (system + microphone).
type Source interface {
	Start(ctx context.Context) (<-chan frame.Dual, error)
	Stop() error
}

// sinkClient is the subset of *parec.Client that capture depends on.
// Narrowed for testability; *parec.Client satisfies it structurally.
type sinkClient interface {
	ListSinks(ctx context.Context, req parec.ListSinksRequest) (parec.ListSinksResponse, error)
	ListSources(ctx context.Context, req parec.ListSourcesRequest) (parec.ListSourcesResponse, error)
	StartCapture(ctx context.Context, req parec.StartCaptureRequest) (*parec.CaptureStream, error)
}
