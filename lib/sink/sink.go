package sink

import (
	"context"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

type Sink interface {
	ID() string
	Write(ctx context.Context, records []dns.Record) error
}
