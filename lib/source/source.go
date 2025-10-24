package source

import (
	"context"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

type Source interface {
	Type() string
	Name() string
	Read(ctx context.Context) ([]dns.Record, error)
}
