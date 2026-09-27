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

func serve(t *testing.T, recs []dns.Record) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(dns.Records{Records: recs})
	}))
	t.Cleanup(srv.Close)

	return srv
}

var upstream = []dns.Record{
	{Type: dns.A, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1"), Source: "upstream"},
	{Type: dns.A, Name: "b.example.org.", TTL: 60, Address: netip.MustParseAddr("10.0.0.2")},
}

func TestRead(t *testing.T) {
	srv := serve(t, upstream)

	h, err := New(slog.Default(), Config{URL: srv.URL}, "http.test")
	require.NoError(t, err)
	require.Equal(t, "http.test", h.ID())

	recs, err := h.Read(context.Background())
	require.NoError(t, err)
	require.Equal(t, []dns.Record{
		{Type: dns.A, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1"), Source: "http.test"},
		{Type: dns.A, Name: "b.example.org.", TTL: 60, Address: netip.MustParseAddr("10.0.0.2"), Source: "http.test"},
	}, recs)
}

func TestReadKeepOriginalSource(t *testing.T) {
	srv := serve(t, upstream)

	h, err := New(slog.Default(), Config{URL: srv.URL, KeepOriginalSource: true}, "http.test")
	require.NoError(t, err)

	recs, err := h.Read(context.Background())
	require.NoError(t, err)
	require.Len(t, recs, 2)
	require.Equal(t, "upstream", recs[0].Source)
	require.Equal(t, "http.test", recs[1].Source)
}

func TestReadFilter(t *testing.T) {
	srv := serve(t, upstream)

	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`filter: {accept: 'hasSuffix(record.name, ".example.com.")'}`), &cfg))
	cfg.URL = srv.URL

	h, err := New(slog.Default(), cfg, "http.test")
	require.NoError(t, err)

	recs, err := h.Read(context.Background())
	require.NoError(t, err)
	require.Len(t, recs, 1)
	require.Equal(t, "a.example.com.", recs[0].Name)
}

func TestReadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	h, err := New(slog.Default(), Config{URL: srv.URL}, "http.test")
	require.NoError(t, err)

	_, err = h.Read(context.Background())
	require.Error(t, err)
}

func TestReadTimeout(t *testing.T) {
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	defer srv.Close()
	defer close(done)

	h, err := New(slog.Default(), Config{URL: srv.URL, Timeout: 50 * time.Millisecond}, "http.test")
	require.NoError(t, err)

	_, err = h.Read(context.Background())
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestReadUnsupportedType(t *testing.T) {
	srv := serve(t, []dns.Record{{Type: "TXT", Name: "a.example.com."}})

	h, err := New(slog.Default(), Config{URL: srv.URL}, "http.test")
	require.NoError(t, err)

	_, err = h.Read(context.Background())
	require.ErrorContains(t, err, "TXT")
}
