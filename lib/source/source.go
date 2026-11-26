package source

import (
	"context"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

type Source interface {
	ID() string
	Read(ctx context.Context) ([]dns.Record, error)
}

// Notifier is implemented by sources that can tell when their data changed,
// so that the caller reads them at once instead of waiting for its next
// periodic read. The channel gets a value for each change (values may be
// dropped when the reader is slow) and is closed when ctx is done. A nil
// channel means the source can't notify; the periodic read still applies.
type Notifier interface {
	Changes(ctx context.Context) <-chan struct{}
}
