package traefik

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/ShimmerGlass/shimdns/lib/rest"
	"github.com/samber/lo"
)

const Type = "traefik"

type Traefik struct {
	log *slog.Logger
	cfg Config
	id  string

	allowedEntrypoints map[string]bool
}

func New(log *slog.Logger, cfg Config, id string) (*Traefik, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	if cfg.Mode == "" {
		cfg.Mode = modeAddress
	}

	return &Traefik{
		log: log,
		cfg: cfg,
		id:  id,
		allowedEntrypoints: lo.SliceToMap(cfg.Entrypoints, func(v string) (string, bool) {
			return v, true
		}),
	}, nil
}

func (t *Traefik) ID() string {
	return t.id
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
	epHTTP2 := map[string]bool{}
	epHTTP3 := map[string]bool{}
	for _, ep := range eps {
		addrPort, err := netip.ParseAddrPort(ep.Address)
		if err != nil {
			return nil, fmt.Errorf("traefik: entrypoint %s: %w", ep.Name, err)
		}

		epToAddr[ep.Name] = addrPort.Addr()
		epHTTP2[ep.Name] = ep.HTTP2 != nil
		epHTTP3[ep.Name] = ep.HTTP3 != nil
	}

	res := []dns.Record{}

	// a host served by several routers gets one record per address and a
	// single HTTPS record with the ALPNs of all its TLS routers
	seenAddrs := map[string]map[netip.Addr]bool{}
	httpsRecs := map[string]*dns.Record{}
	var httpsHosts []string

	for _, router := range routers {
		for host := range routersHosts(router) {
			host = dns.NormName(host)

			if seenAddrs[host] == nil {
				seenAddrs[host] = map[netip.Addr]bool{}
			}

			alpn := []string{dns.AlpnHTTP11}
			resolved := false
			for _, ep := range router.EntryPoints {
				if !t.entrypointAllowed(ep) {
					continue
				}

				if epHTTP2[ep] {
					alpn = append(alpn, dns.AlpnHTTP2)
				}
				if epHTTP3[ep] {
					alpn = append(alpn, dns.AlpnHTTP3)
				}

				addrs := t.cfg.Addresses
				epAddr, ok := epToAddr[ep]
				if len(addrs) == 0 && ok {
					addrs = append(addrs, epAddr)
				}

				for _, addr := range addrs {
					resolved = true

					if seenAddrs[host][addr] {
						continue
					}

					seenAddrs[host][addr] = true

					rec := dns.Record{
						Name:    host,
						TTL:     t.cfg.TTL,
						Address: addr,
						Source:  t.id,
					}

					if addr.Is4() {
						rec.Type = dns.A
					} else {
						rec.Type = dns.AAAA
					}

					res = append(res, rec)
				}
			}

			// only advertise HTTPS for TLS hosts that resolve through this source,
			// whatever issues their certificate
			if router.TLS == nil || !resolved {
				continue
			}

			httpsRec, ok := httpsRecs[host]
			if !ok {
				httpsRec = &dns.Record{
					Type:     dns.HTTPS,
					Target:   ".",
					TTL:      t.cfg.TTL,
					Priority: 1,
					Name:     host,
					Source:   t.id,
				}
				httpsRecs[host] = httpsRec
				httpsHosts = append(httpsHosts, host)
			}

			httpsRec.Alpn = append(httpsRec.Alpn, alpn...)
		}
	}

	for _, host := range httpsHosts {
		rec := httpsRecs[host]
		rec.Alpn = sortAlpn(lo.Uniq(rec.Alpn))
		res = append(res, *rec)
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
		if !slices.ContainsFunc(router.EntryPoints, t.entrypointAllowed) {
			continue
		}

		for host := range routersHosts(router) {
			res = append(res, dns.Record{
				Type:   dns.CNAME,
				Name:   dns.NormName(host),
				TTL:    t.cfg.TTL,
				Target: t.cfg.Target,
				Source: t.id,
			})
		}
	}

	return res, nil
}

// entrypointAllowed reports whether routers on the entrypoint should produce
// records. All entrypoints are allowed when none are configured.
func (t *Traefik) entrypointAllowed(ep string) bool {
	return len(t.allowedEntrypoints) == 0 || t.allowedEntrypoints[ep]
}

func (t *Traefik) entrypoints(ctx context.Context) ([]entrypoint, error) {
	return getAll[entrypoint](ctx, t.cfg.URL, "/api/entrypoints")
}

func (t *Traefik) routers(ctx context.Context) ([]router, error) {
	return getAll[router](ctx, t.cfg.URL, "/api/http/routers")
}

// maxPages bounds getAll in case the API keeps announcing a next page.
const maxPages = 100

// getAll reads every page of a traefik API list: traefik returns 100 items
// per page by default and sets X-Next-Page to the next page number, or to 1
// on the last page.
func getAll[T any](ctx context.Context, baseURL, path string) ([]T, error) {
	var all []T

	page := 1
	for range maxPages {
		var hdr http.Header
		items, err := rest.Get[[]T](ctx, rest.Request{
			URL:            baseURL,
			Path:           path,
			Query:          url.Values{"page": {strconv.Itoa(page)}},
			ResponseHeader: &hdr,
		})
		if err != nil {
			return nil, err
		}

		all = append(all, items...)

		next, err := strconv.Atoi(hdr.Get("X-Next-Page"))
		if err != nil || next <= page {
			return all, nil
		}
		page = next
	}

	return nil, fmt.Errorf("traefik: %s: more than %d pages", path, maxPages)
}

var hostReg = regexp.MustCompile("Host\\(['\"`]([^'\"`]+)['\"`]\\)")

var alpnOrder = []string{dns.AlpnHTTP11, dns.AlpnHTTP2, dns.AlpnHTTP3}

// sortAlpn orders ALPNs the same way whatever the routers' order, so the
// record doesn't change between reads
func sortAlpn(alpn []string) []string {
	slices.SortFunc(alpn, func(a, b string) int {
		return slices.Index(alpnOrder, a) - slices.Index(alpnOrder, b)
	})
	return alpn
}

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
