package sink

import (
	"context"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

type Sink interface {
	Write(ctx context.Context, records []dns.Record) error
}
