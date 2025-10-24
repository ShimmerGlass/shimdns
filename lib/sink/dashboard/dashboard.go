package dashboard

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

type Dashboard struct {
	log *slog.Logger
	cfg Config

	lock    sync.Mutex
	records []dns.Record
}

func New(log *slog.Logger, cfg Config, mux *http.ServeMux) (*Dashboard, error) {
	d := &Dashboard{
		log: log.With("sink", "dashboard"),
		cfg: cfg,
	}

	d.register(mux)

	return d, nil
}

func (d *Dashboard) Write(ctx context.Context, recs []dns.Record) error {
	d.lock.Lock()
	d.records = make([]dns.Record, len(recs))
	copy(d.records, recs)

	slices.SortFunc(d.records, func(a, b dns.Record) int {
		if a.Name == b.Name {
			return strings.Compare(string(a.Type), string(b.Type))
		}

		return strings.Compare(a.Name, b.Name)
	})
	d.lock.Unlock()

	return nil
}

func (d *Dashboard) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		d.lock.Lock()
		recs := d.records
		d.lock.Unlock()

		index(recs).Render(r.Context(), w)
	})
}
