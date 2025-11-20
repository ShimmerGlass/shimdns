package source

import (
	"context"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

type Source interface {
	ID() string
	Read(ctx context.Context) ([]dns.Record, error)
}
