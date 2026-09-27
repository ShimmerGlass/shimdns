package dashboard

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
)

func TestWriteSorts(t *testing.T) {
	d, err := New(slog.Default(), Config{}, "dashboard.test", http.NewServeMux())
	require.NoError(t, err)
	require.Equal(t, "dashboard.test", d.ID())

	in := []dns.Record{
		{Type: dns.CNAME, Name: "c.example.com.", Target: "a.example.com."},
		{Type: dns.AAAA, Name: "a.example.com.", Address: netip.MustParseAddr("2001:db8::1")},
		{Type: dns.A, Name: "b.example.com.", Address: netip.MustParseAddr("10.0.0.2")},
		{Type: dns.A, Name: "a.example.com.", Address: netip.MustParseAddr("10.0.0.1")},
	}
	orig := append([]dns.Record{}, in...)

	require.NoError(t, d.Write(context.Background(), in))

	// the input slice must not be modified
	require.Equal(t, orig, in)

	require.Equal(t, []dns.Record{orig[3], orig[1], orig[2], orig[0]}, d.records)
}

func TestServe(t *testing.T) {
	mux := http.NewServeMux()

	d, err := New(slog.Default(), Config{}, "dashboard.test", mux)
	require.NoError(t, err)

	require.NoError(t, d.Write(context.Background(), []dns.Record{
		{Type: dns.A, Name: "a.example.com.", Address: netip.MustParseAddr("10.0.0.1"), Source: "file.static"},
		{Type: dns.CNAME, Name: "b.example.com.", Target: "a.example.com.", Source: "traefik.proxy"},
	}))

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	for _, s := range []string{"a.example.com.", "10.0.0.1", "file.static", "b.example.com.", "traefik.proxy"} {
		require.True(t, strings.Contains(body, s), "missing %q in body", s)
	}
}
