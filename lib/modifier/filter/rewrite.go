package filter

import (
	"context"
	"log/slog"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

type Filter struct {
	log *slog.Logger
	cfg Config
}

func New(log *slog.Logger, cfg Config) (*Filter, error) {
	return &Filter{
		log: log.With("modifier", "filter"),
		cfg: cfg,
	}, nil
}

func (p *Filter) Modify(ctx context.Context, records []dns.Record) ([]dns.Record, error) {
	res := make([]dns.Record, 0, len(records))

	for _, rec := range records {
		ok, err := p.cfg.Filter.Match(rec)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		res = append(res, rec)
	}

	return res, nil
}
