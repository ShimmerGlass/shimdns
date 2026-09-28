package traefik

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strconv"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const entrypointsJSON = `[
	{"address": "10.0.0.1:80", "name": "web"},
	{"address": "10.0.0.1:443", "name": "websecure", "http2": {"maxConcurrentStreams": 250}, "http3": {}},
	{"address": "[2001:db8::1]:8443", "name": "v6", "http2": {"maxConcurrentStreams": 250}}
]`

const routersJSON = `[
	{
		"entryPoints": ["websecure"],
		"rule": "Host(` + "`a.example.com`" + `) || Host(\"b.example.com\")",
		"tls": {"certResolver": "le"},
		"name": "a@docker"
	},
	{
		"entryPoints": ["web"],
		"rule": "Host('c.example.com') && PathPrefix(` + "`/api`" + `)",
		"name": "c@docker"
	},
	{
		"entryPoints": ["web", "websecure"],
		"rule": "Host(` + "`d.example.com`" + `)",
		"name": "d@docker"
	},
	{
		"entryPoints": ["v6"],
		"rule": "Host(` + "`e.example.com`" + `)",
		"name": "e@docker"
	},
	{
		"entryPoints": ["web", "websecure"],
		"rule": "Host(` + "`f.example.com`" + `)",
		"tls": {"certResolver": "le"},
		"name": "f@docker"
	},
	{
		"entryPoints": ["web"],
		"rule": "PathPrefix(` + "`/metrics`" + `)",
		"name": "nohost@docker"
	}
]`

func serve(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entrypoints":
			_, _ = w.Write([]byte(entrypointsJSON))
		case "/api/http/routers":
			_, _ = w.Write([]byte(routersJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func sortRecs(recs []dns.Record) {
	slices.SortFunc(recs, func(a, b dns.Record) int {
		if c := compareStr(a.Name, b.Name); c != 0 {
			return c
		}
		if c := compareStr(a.Type, b.Type); c != 0 {
			return c
		}
		return a.Address.Compare(b.Address)
	})
}

func compareStr(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func a(name, addr string) dns.Record {
	r := dns.Record{Name: name, TTL: 60, Address: netip.MustParseAddr(addr), Source: "traefik.test"}
	if r.Address.Is4() {
		r.Type = dns.A
	} else {
		r.Type = dns.AAAA
	}
	return r
}

func TestReadAddress(t *testing.T) {
	srv := serve(t)

	tr, err := New(slog.Default(), Config{URL: srv.URL, TTL: 60}, "traefik.test")
	require.NoError(t, err)
	require.Equal(t, "traefik.test", tr.ID())

	recs, err := tr.Read(context.Background())
	require.NoError(t, err)

	want := []dns.Record{
		a("a.example.com.", "10.0.0.1"),
		{Type: dns.HTTPS, Name: "a.example.com.", TTL: 60, Priority: 1, Target: ".", Alpn: []string{dns.AlpnHTTP11, dns.AlpnHTTP2, dns.AlpnHTTP3}, Source: "traefik.test"},
		a("b.example.com.", "10.0.0.1"),
		{Type: dns.HTTPS, Name: "b.example.com.", TTL: 60, Priority: 1, Target: ".", Alpn: []string{dns.AlpnHTTP11, dns.AlpnHTTP2, dns.AlpnHTTP3}, Source: "traefik.test"},
		a("c.example.com.", "10.0.0.1"),
		// two entrypoints on the same address: a single record
		a("d.example.com.", "10.0.0.1"),
		a("e.example.com.", "2001:db8::1"),
		a("f.example.com.", "10.0.0.1"),
		{Type: dns.HTTPS, Name: "f.example.com.", TTL: 60, Priority: 1, Target: ".", Alpn: []string{dns.AlpnHTTP11, dns.AlpnHTTP2, dns.AlpnHTTP3}, Source: "traefik.test"},
	}

	sortRecs(recs)
	sortRecs(want)
	require.Equal(t, want, recs)
}

func TestReadAddressStaticAddresses(t *testing.T) {
	srv := serve(t)

	tr, err := New(slog.Default(), Config{
		URL:       srv.URL,
		TTL:       60,
		Addresses: []netip.Addr{netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("2001:db8::2")},
	}, "traefik.test")
	require.NoError(t, err)

	recs, err := tr.Read(context.Background())
	require.NoError(t, err)

	var got []dns.Record
	for _, r := range recs {
		if r.Name == "c.example.com." {
			got = append(got, r)
		}
	}

	sortRecs(got)
	require.Equal(t, []dns.Record{
		a("c.example.com.", "192.168.1.1"),
		a("c.example.com.", "2001:db8::2"),
	}, got)
}

func TestReadAddressEntrypoints(t *testing.T) {
	srv := serve(t)

	tr, err := New(slog.Default(), Config{URL: srv.URL, TTL: 60, Entrypoints: []string{"v6"}}, "traefik.test")
	require.NoError(t, err)

	recs, err := tr.Read(context.Background())
	require.NoError(t, err)
	require.Equal(t, []dns.Record{a("e.example.com.", "2001:db8::1")}, recs)
}

func TestReadAddressInvalidEntrypoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entrypoints":
			_, _ = w.Write([]byte(`[{"address": "garbage", "name": "web"}]`))
		case "/api/http/routers":
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()

	tr, err := New(slog.Default(), Config{URL: srv.URL}, "traefik.test")
	require.NoError(t, err)

	_, err = tr.Read(context.Background())
	require.ErrorContains(t, err, "web")
}

func TestReadCname(t *testing.T) {
	srv := serve(t)

	tr, err := New(slog.Default(), Config{
		URL:         srv.URL,
		TTL:         60,
		Mode:        modeCname,
		Target:      "proxy.example.com.",
		Entrypoints: []string{"websecure"},
	}, "traefik.test")
	require.NoError(t, err)

	recs, err := tr.Read(context.Background())
	require.NoError(t, err)

	cname := func(name string) dns.Record {
		return dns.Record{Type: dns.CNAME, Name: name, TTL: 60, Target: "proxy.example.com.", Source: "traefik.test"}
	}

	require.Equal(t, []dns.Record{
		cname("a.example.com."),
		cname("b.example.com."),
		cname("d.example.com."),
		cname("f.example.com."),
	}, recs)
}

func TestReadCnameAllEntrypoints(t *testing.T) {
	srv := serve(t)

	tr, err := New(slog.Default(), Config{URL: srv.URL, Mode: modeCname, Target: "proxy.example.com."}, "traefik.test")
	require.NoError(t, err)

	recs, err := tr.Read(context.Background())
	require.NoError(t, err)

	var names []string
	for _, r := range recs {
		require.Equal(t, dns.CNAME, r.Type)
		names = append(names, r.Name)
	}
	require.Equal(t, []string{"a.example.com.", "b.example.com.", "c.example.com.", "d.example.com.", "e.example.com.", "f.example.com."}, names)
}

func TestReadAddressAlpnFromAllowedEntrypoints(t *testing.T) {
	srv := serve(t)

	// f is on web and websecure, only web is allowed: h2/h3 come from websecure
	tr, err := New(slog.Default(), Config{URL: srv.URL, TTL: 60, Entrypoints: []string{"web"}}, "traefik.test")
	require.NoError(t, err)

	recs, err := tr.Read(context.Background())
	require.NoError(t, err)

	var https []dns.Record
	for _, r := range recs {
		if r.Type == dns.HTTPS {
			https = append(https, r)
		}
	}

	require.Equal(t, []dns.Record{
		{Type: dns.HTTPS, Name: "f.example.com.", TTL: 60, Priority: 1, Target: ".", Alpn: []string{dns.AlpnHTTP11}, Source: "traefik.test"},
	}, https)
}

func TestReadFilter(t *testing.T) {
	srv := serve(t)

	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`filter: {reject: 'record.type == "HTTPS"'}`), &cfg))
	cfg.URL = srv.URL

	tr, err := New(slog.Default(), cfg, "traefik.test")
	require.NoError(t, err)

	recs, err := tr.Read(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, recs)
	for _, r := range recs {
		require.NotEqual(t, dns.HTTPS, r.Type)
	}
}

func TestReadInvalidMode(t *testing.T) {
	tr, err := New(slog.Default(), Config{Mode: "nope"}, "traefik.test")
	require.NoError(t, err)

	_, err = tr.Read(context.Background())
	require.ErrorContains(t, err, "nope")
}

func TestReadAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	for _, mode := range []string{modeAddress, modeCname} {
		tr, err := New(slog.Default(), Config{URL: srv.URL, Mode: mode, Entrypoints: []string{"web"}}, "traefik.test")
		require.NoError(t, err)

		_, err = tr.Read(context.Background())
		require.Error(t, err, mode)
	}
}

func TestRoutersHosts(t *testing.T) {
	cases := []struct {
		rule string
		want []string
	}{
		{"Host(`a.example.com`)", []string{"a.example.com"}},
		{`Host("a.example.com")`, []string{"a.example.com"}},
		{`Host('a.example.com')`, []string{"a.example.com"}},
		{"Host(`a.example.com`) || Host(`b.example.com`)", []string{"a.example.com", "b.example.com"}},
		{"Host(`a.example.com`) && PathPrefix(`/api`)", []string{"a.example.com"}},
		{"PathPrefix(`/api`)", nil},
		{"HostRegexp(`^.+\\.example\\.com$`)", nil},
	}

	for _, tc := range cases {
		var got []string
		for h := range routersHosts(router{Rule: tc.rule}) {
			got = append(got, h)
		}
		require.Equal(t, tc.want, got, tc.rule)
	}
}

func TestModelsDecode(t *testing.T) {
	var eps []entrypoint
	require.NoError(t, json.Unmarshal([]byte(entrypointsJSON), &eps))
	require.Nil(t, eps[0].HTTP2)
	require.Nil(t, eps[0].HTTP3)
	require.NotNil(t, eps[1].HTTP2)
	require.NotNil(t, eps[1].HTTP3)
}

// traefik pages its API lists (100 items by default): X-Next-Page is the next
// page, 1 on the last one.
func TestReadAddressPaginated(t *testing.T) {
	var routers []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(routersJSON), &routers))
	pages := [][]json.RawMessage{routers[:2], routers[2:4], routers[4:]}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entrypoints":
			w.Header().Set("X-Next-Page", "1")
			_, _ = w.Write([]byte(entrypointsJSON))
		case "/api/http/routers":
			page := 1
			if p := r.URL.Query().Get("page"); p != "" {
				page = int(p[0] - '0')
			}
			next := page + 1
			if next > len(pages) {
				next = 1
			}
			w.Header().Set("X-Next-Page", string(rune('0'+next)))
			_ = json.NewEncoder(w).Encode(pages[page-1])
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	paged, err := New(slog.Default(), Config{URL: srv.URL, TTL: 60}, "traefik.test")
	require.NoError(t, err)
	recs, err := paged.Read(context.Background())
	require.NoError(t, err)

	single, err := New(slog.Default(), Config{URL: serve(t).URL, TTL: 60}, "traefik.test")
	require.NoError(t, err)
	want, err := single.Read(context.Background())
	require.NoError(t, err)

	sortRecs(recs)
	sortRecs(want)
	require.Equal(t, want, recs)
}

func TestReadPaginationLoop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// always announces a further page
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		w.Header().Set("X-Next-Page", strconv.Itoa(page+1))
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)

	_, err := getAll[router](context.Background(), srv.URL, "/api/http/routers")
	require.Error(t, err)
}
