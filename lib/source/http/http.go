package http

import (
	"context"
	"log/slog"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/ShimmerGlass/shimdns/lib/rest"
)

const Type = "http"

type HTTP struct {
	log *slog.Logger
	cfg Config
}

func New(log *slog.Logger, cfg Config) (*HTTP, error) {
	if cfg.Name == "" {
		cfg.Name = cfg.URL
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	return &HTTP{
		log: log.With("source", Type, "source_name", cfg.Name),
		cfg: cfg,
	}, nil
}

func (h *HTTP) Type() string {
	return Type
}

func (h *HTTP) Name() string {
	return h.cfg.Name
}

func (h *HTTP) Read(ctx context.Context) ([]dns.Record, error) {
	ctx, cancel := context.WithTimeout(ctx, h.cfg.Timeout)
	defer cancel()

	res, err := rest.Get[dns.Records](ctx, rest.Request{
		URL: h.cfg.URL,
	})
	if err != nil {
		return nil, err
	}

	recs := res.Records[:0]
	for _, rec := range res.Records {
		ok, err := h.cfg.Filter.Match(rec)
		if err != nil {
			return nil, err
		}

		if !ok {
			h.log.Debug("filter drop", "record", rec)
			continue
		}

		if rec.Source == "" || !h.cfg.KeepOriginalSource {
			rec.Source = Type
			rec.SourceName = h.cfg.Name
		}

		recs = append(recs, rec)
	}

	return recs, nil
}
