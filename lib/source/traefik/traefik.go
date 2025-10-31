package traefik

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"net/netip"
	"regexp"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/ShimmerGlass/shimdns/lib/rest"
	"github.com/samber/lo"
)

const Type = "traefik"

type Traefik struct {
	log *slog.Logger
	cfg Config

	allowedEntrypoints map[string]bool
}

func New(log *slog.Logger, cfg Config) (*Traefik, error) {
	if cfg.Name == "" {
		cfg.Name = cfg.URL
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	if cfg.Mode == "" {
		cfg.Mode = modeAddress
	}

	return &Traefik{
		log: log.With("source", Type, "source_name", cfg.Name),
		cfg: cfg,
		allowedEntrypoints: lo.SliceToMap(cfg.Entrypoints, func(v string) (string, bool) {
			return v, true
		}),
	}, nil
}

func (t *Traefik) Type() string {
	return Type
}

func (t *Traefik) Name() string {
	return t.cfg.Name
}

func (t *Traefik) Read(ctx context.Context) ([]dns.Record, error) {
	var recs []dns.Record
	var err error

	switch t.cfg.Mode {
	case modeAddress:
		recs, err = t.readAddress(ctx)

	case modeCname:
		recs, err = t.readCname(ctx)

	default:
		return nil, fmt.Errorf("invalid mode %q", t.cfg.Mode)
	}

	if err != nil {
		return nil, err
	}

	recs = lo.Uniq(recs)
	recs, err = t.cfg.Filter.Filter(recs)
	if err != nil {
		return nil, err
	}

	return recs, nil
}

func (t *Traefik) readAddress(ctx context.Context) ([]dns.Record, error) {
	eps, err := t.entrypoints(ctx)
	if err != nil {
		return nil, err
	}

	routers, err := t.routers(ctx)
	if err != nil {
		return nil, err
	}

	epToAddr := map[string]netip.Addr{}
	for _, ep := range eps {
		addrPort, err := netip.ParseAddrPort(ep.Address)
		if err != nil {
			return nil, fmt.Errorf("traefik: entrypoint %s: %w", ep.Name, err)
		}

		epToAddr[ep.Name] = addrPort.Addr()
	}

	res := []dns.Record{}

	for _, router := range routers {
		for host := range routersHosts(router) {
			host = dns.NormName(host)

			for _, ep := range router.EntryPoints {
				if _, ok := t.allowedEntrypoints[ep]; len(t.allowedEntrypoints) > 0 && !ok {
					continue
				}

				addrs := t.cfg.Addresses
				epAddr, ok := epToAddr[ep]
				if len(addrs) == 0 && ok {
					addrs = append(addrs, epAddr)
				}

				for _, addr := range addrs {
					rec := dns.Record{
						Name:       host,
						Address:    addr,
						Source:     Type,
						SourceName: t.cfg.Name,
					}

					if addr.Is4() {
						rec.Type = dns.A
					} else {
						rec.Type = dns.AAAA
					}

					res = append(res, rec)
				}
			}
		}

		return res, nil
	}

	return res, nil
}

func (t *Traefik) readCname(ctx context.Context) ([]dns.Record, error) {
	routers, err := t.routers(ctx)
	if err != nil {
		return nil, err
	}

	res := []dns.Record{}

	for _, router := range routers {
		epOK := false
		for _, ep := range router.EntryPoints {
			if t.allowedEntrypoints[ep] {
				epOK = true
				break
			}
		}
		if !epOK {
			continue
		}

		for host := range routersHosts(router) {
			res = append(res, dns.Record{
				Type:       dns.CNAME,
				Name:       dns.NormName(host),
				Target:     t.cfg.Target,
				Source:     Type,
				SourceName: t.cfg.Name,
			})
		}
	}

	return res, nil
}

func (t *Traefik) entrypoints(ctx context.Context) ([]entrypoint, error) {
	return rest.Get[[]entrypoint](ctx, rest.Request{
		URL:  t.cfg.URL,
		Path: "/api/entrypoints",
	})
}

func (t *Traefik) routers(ctx context.Context) ([]router, error) {
	return rest.Get[[]router](ctx, rest.Request{
		URL:  t.cfg.URL,
		Path: "/api/http/routers",
	})
}

var hostReg = regexp.MustCompile("Host\\(['\"`]([^'\"`]+)['\"`]\\)")

func routersHosts(router router) iter.Seq[string] {
	matches := hostReg.FindAllStringSubmatch(router.Rule, -1)

	return func(yield func(string) bool) {
		for _, g := range matches {
			if !yield(g[1]) {
				return
			}
		}
	}
}
