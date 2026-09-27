package mikrotik

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type fakeRouter struct {
	lock    sync.Mutex
	entries []entry
	added   []entry
	deleted []string
}

func (f *fakeRouter) serve(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		f.lock.Lock()
		defer f.lock.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/ip/dns/static":
			_ = json.NewEncoder(w).Encode(f.entries)

		case r.Method == http.MethodPut && r.URL.Path == "/rest/ip/dns/static":
			var e entry
			require.NoError(t, json.NewDecoder(r.Body).Decode(&e))
			f.added = append(f.added, e)
			_ = json.NewEncoder(w).Encode(e)

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/rest/ip/dns/static/"):
			f.deleted = append(f.deleted, strings.TrimPrefix(r.URL.Path, "/rest/ip/dns/static/"))
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func newTestSink(t *testing.T, cfg Config, router *fakeRouter) *Mikrotik {
	t.Helper()

	srv := router.serve(t)
	cfg.URL = srv.URL
	cfg.User = "admin"
	cfg.Password = "secret"

	m, err := New(slog.Default(), cfg, "mikrotik.test")
	require.NoError(t, err)
	require.Equal(t, "mikrotik.test", m.ID())

	return m
}

func existingEntries() []entry {
	return []entry{
		{ID: "*1", Type: dns.A, Name: "a.example.com", Address: "10.0.0.1", Comment: "shimdns"},
		{ID: "*2", Type: dns.A, Name: "old.example.com", Address: "10.0.0.9", Comment: "shimdns"},
		{ID: "*3", Type: dns.AAAA, Name: "a.example.com", Address: "2001:db8::1", Comment: "shimdns"},
		{ID: "*4", Type: dns.A, Name: "manual.example.com", Address: "10.0.0.10", Comment: "manual"},
	}
}

var testRecords = []dns.Record{
	{Type: dns.A, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1")},
	{Type: dns.AAAA, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("2001:db8::1")},
	{Type: dns.A, Name: "b.example.com.", TTL: 300, Address: netip.MustParseAddr("10.0.0.2")},
	{Type: dns.AAAA, Name: "b.example.com.", TTL: 300, Address: netip.MustParseAddr("2001:db8::2")},
	// PTR records are not supported by mikrotik and must be ignored
	{Type: dns.PTR, Name: "2.0.0.10.in-addr.arpa.", TTL: 60, Ptr: "b.example.com."},
}

func TestWrite(t *testing.T) {
	router := &fakeRouter{entries: existingEntries()}
	m := newTestSink(t, Config{MatchComment: true, Comment: "shimdns"}, router)

	require.NoError(t, m.Write(context.Background(), testRecords))

	require.Equal(t, []string{"*2"}, router.deleted)

	require.Len(t, router.added, 2)
	for i, want := range []entry{
		{Type: dns.A, Address: "10.0.0.2", TTL: "300", Comment: "shimdns", Disabled: "false"},
		{Type: dns.AAAA, Address: "2001:db8::2", TTL: "300", Comment: "shimdns", Disabled: "false"},
	} {
		got := router.added[i]
		require.Empty(t, got.ID)
		require.Contains(t, got.Name, "b.example.com")

		got.Name = ""
		require.Equal(t, want, got)
	}
}

func TestWriteNoChange(t *testing.T) {
	router := &fakeRouter{entries: []entry{
		{ID: "*1", Type: dns.A, Name: "a.example.com", Address: "10.0.0.1", Comment: "shimdns"},
	}}
	m := newTestSink(t, Config{MatchComment: true, Comment: "shimdns"}, router)

	require.NoError(t, m.Write(context.Background(), testRecords[:1]))
	require.Empty(t, router.deleted)
	require.Empty(t, router.added)
}

func TestWriteFilter(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`
match_comment: true
comment: shimdns
filter: {accept: 'record.type == "A"'}
`), &cfg))

	router := &fakeRouter{entries: existingEntries()}
	m := newTestSink(t, cfg, router)

	require.NoError(t, m.Write(context.Background(), testRecords))

	require.Len(t, router.added, 1)
	require.Equal(t, "10.0.0.2", router.added[0].Address)
}

func TestWriteAPIError(t *testing.T) {
	router := &fakeRouter{}
	m := newTestSink(t, Config{}, router)
	m.api.password = "wrong"

	err := m.Write(context.Background(), testRecords)
	require.ErrorContains(t, err, "401")
}

func TestWriteUnsupportedTypes(t *testing.T) {
	router := &fakeRouter{entries: existingEntries()}
	m := newTestSink(t, Config{MatchComment: true, Comment: "shimdns"}, router)

	recs := append([]dns.Record{}, testRecords...)
	recs = append(recs,
		dns.Record{Type: dns.HTTPS, Name: "a.example.com.", TTL: 60, Priority: 1, Target: "."},
		dns.Record{Type: dns.CNAME, Name: "c.example.com.", TTL: 60, Target: "a.example.com."},
		dns.Record{Type: dns.SRV, Name: "_x._tcp.example.com.", TTL: 60, Target: "a.example.com."},
		dns.Record{Type: dns.MX, Name: "example.com.", TTL: 60, Mx: "a.example.com."},
	)

	require.NoError(t, m.Write(context.Background(), recs))
	require.Equal(t, []string{"*2"}, router.deleted)
	require.Len(t, router.added, 2)
}

func TestWriteFilterRemovesExcluded(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`
match_comment: true
comment: shimdns
filter: {accept: 'record.type == "A"'}
`), &cfg))

	router := &fakeRouter{entries: existingEntries()}
	m := newTestSink(t, cfg, router)

	require.NoError(t, m.Write(context.Background(), testRecords))

	// the AAAA entry is excluded by the filter, so the sink no longer owns it
	require.ElementsMatch(t, []string{"*2", "*3"}, router.deleted)
}
