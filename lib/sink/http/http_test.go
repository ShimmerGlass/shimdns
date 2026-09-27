package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var testRecords = []dns.Record{
	{Type: dns.A, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1"), Source: "src"},
	{Type: dns.CNAME, Name: "b.example.com.", TTL: 60, Target: "a.example.com.", Source: "src"},
}

func get(t *testing.T, mux *http.ServeMux, path string) (int, dns.Records) {
	t.Helper()

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))

	var res dns.Records
	if w.Code == http.StatusOK {
		require.NoError(t, json.NewDecoder(w.Body).Decode(&res))
	}

	return w.Code, res
}

func TestServe(t *testing.T) {
	mux := http.NewServeMux()

	h, err := New(slog.Default(), Config{Path: "/records"}, "http.test", mux)
	require.NoError(t, err)
	require.Equal(t, "http.test", h.ID())

	require.NoError(t, h.Write(context.Background(), testRecords))

	code, res := get(t, mux, "/records")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, testRecords, res.Records)

	code, _ = get(t, mux, "/other")
	require.Equal(t, http.StatusNotFound, code)
}

func TestServeLatestWrite(t *testing.T) {
	mux := http.NewServeMux()

	h, err := New(slog.Default(), Config{Path: "/records"}, "http.test", mux)
	require.NoError(t, err)

	require.NoError(t, h.Write(context.Background(), testRecords))
	require.NoError(t, h.Write(context.Background(), testRecords[:1]))

	_, res := get(t, mux, "/records")
	require.Equal(t, testRecords[:1], res.Records)
}

func TestFilter(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`
path: /records
filter: {accept: 'record.type == "CNAME"'}
`), &cfg))

	mux := http.NewServeMux()

	h, err := New(slog.Default(), cfg, "http.test", mux)
	require.NoError(t, err)

	require.NoError(t, h.Write(context.Background(), testRecords))

	_, res := get(t, mux, "/records")
	require.Equal(t, testRecords[1:], res.Records)
}

// The JSON produced by the sink must be readable by the http source.
func TestRoundTrip(t *testing.T) {
	all := []dns.Record{
		{Type: dns.A, Name: "a.", TTL: 1, Address: netip.MustParseAddr("10.0.0.1"), Source: "s"},
		{Type: dns.AAAA, Name: "a.", TTL: 1, Address: netip.MustParseAddr("2001:db8::1"), Source: "s"},
		{Type: dns.PTR, Name: "1.0.0.10.in-addr.arpa.", TTL: 1, Ptr: "a.", Source: "s"},
		{Type: dns.SRV, Name: "_x._tcp.", TTL: 1, Priority: 1, Weight: 2, Port: 3, Target: "a.", Source: "s"},
		{Type: dns.MX, Name: "m.", TTL: 1, Preference: 10, Mx: "a.", Source: "s"},
		{Type: dns.HTTPS, Name: "a.", TTL: 1, Priority: 1, Target: ".", Alpn: []string{dns.AlpnHTTP2}, Source: "s"},
	}

	mux := http.NewServeMux()

	h, err := New(slog.Default(), Config{Path: "/records"}, "http.test", mux)
	require.NoError(t, err)
	require.NoError(t, h.Write(context.Background(), all))

	_, res := get(t, mux, "/records")
	require.Equal(t, all, res.Records)
}

func TestFilterErrorDoesNotBlock(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`
path: /records
filter: {accept: 'record.address == ip("bad")'}
`), &cfg))

	mux := http.NewServeMux()

	h, err := New(slog.Default(), cfg, "http.test", mux)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)

		require.Error(t, h.Write(context.Background(), testRecords))
		require.Error(t, h.Write(context.Background(), testRecords))
		get(t, mux, "/records")
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("sink blocked after a filter error")
	}
}
