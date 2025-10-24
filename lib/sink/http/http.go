package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

type HTTP struct {
	log *slog.Logger
	cfg Config

	lock    sync.Mutex
	records []dns.Record
}

func New(log *slog.Logger, cfg Config, mux *http.ServeMux) (*HTTP, error) {
	d := &HTTP{
		log: log.With("sink", "http"),
		cfg: cfg,
	}

	d.register(mux)

	return d, nil
}

func (d *HTTP) Write(ctx context.Context, recs []dns.Record) error {
	d.lock.Lock()
	d.records = []dns.Record{}

	for _, rec := range recs {
		ok, err := d.cfg.Filter.Match(rec)
		if err != nil {
			return err
		}

		if ok {
			d.records = append(d.records, rec)
		}
	}

	d.lock.Unlock()

	return nil
}

func (d *HTTP) register(mux *http.ServeMux) {
	mux.HandleFunc(fmt.Sprintf("GET %s", d.cfg.Path), func(w http.ResponseWriter, r *http.Request) {
		d.lock.Lock()
		recs := d.records
		d.lock.Unlock()

		json.NewEncoder(w).Encode(dns.Records{Records: recs})
	})
}
