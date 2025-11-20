package modifier

import (
	"context"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

type Modifier interface {
	ID() string
	Modify(ctx context.Context, recs []dns.Record) ([]dns.Record, error)
}
