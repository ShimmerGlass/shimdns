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

const Type = "http"

type HTTP struct {
	log *slog.Logger
	cfg Config
	id  string

	lock    sync.Mutex
	records []dns.Record
}

func New(log *slog.Logger, cfg Config, id string, mux *http.ServeMux) (*HTTP, error) {
	d := &HTTP{
		log: log,
		id:  id,
		cfg: cfg,
	}

	d.register(mux)

	return d, nil
}

func (h *HTTP) ID() string {
	return h.id
}

func (h *HTTP) Write(ctx context.Context, recs []dns.Record) error {
	h.lock.Lock()
	h.records = []dns.Record{}

	for _, rec := range recs {
		ok, err := h.cfg.Filter.Match(rec)
		if err != nil {
			return err
		}

		if ok {
			h.records = append(h.records, rec)
		}
	}

	h.lock.Unlock()

	return nil
}

func (h *HTTP) register(mux *http.ServeMux) {
	mux.HandleFunc(fmt.Sprintf("GET %s", h.cfg.Path), func(w http.ResponseWriter, r *http.Request) {
		h.lock.Lock()
		recs := h.records
		h.lock.Unlock()

		_ = json.NewEncoder(w).Encode(dns.Records{Records: recs})
	})
}
