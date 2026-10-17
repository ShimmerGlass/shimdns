package sink

import (
	"context"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

type Sink interface {
	ID() string
	Write(ctx context.Context, records []dns.Record) error
}

// RemovalGracer is implemented by sinks that keep records that disappeared
// from the sources for a while before removing them.
type RemovalGracer interface {
	RemovalGrace() time.Duration
}

// WithRemovalGrace gives snk a removal grace.
func WithRemovalGrace(snk Sink, grace time.Duration) Sink {
	return &graced{Sink: snk, grace: grace}
}

type graced struct {
	Sink
	grace time.Duration
}

func (g *graced) RemovalGrace() time.Duration { return g.grace }
