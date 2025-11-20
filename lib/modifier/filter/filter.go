package filter

import (
	"context"
	"log/slog"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

const Type = "filter"

type Filter struct {
	log *slog.Logger
	cfg Config
	id  string
}

func New(log *slog.Logger, cfg Config, id string) (*Filter, error) {
	return &Filter{
		log: log,
		cfg: cfg,
		id:  id,
	}, nil
}

func (f *Filter) ID() string {
	return f.id
}

func (f *Filter) Modify(ctx context.Context, records []dns.Record) ([]dns.Record, error) {
	res := make([]dns.Record, 0, len(records))

	for _, rec := range records {
		ok, err := f.cfg.Filter.Match(rec)
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
