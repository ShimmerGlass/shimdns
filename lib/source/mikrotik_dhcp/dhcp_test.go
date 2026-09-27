package mikrotikdhcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func serve(t *testing.T, leases []Lease) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/ip/dhcp-server/lease", r.URL.Path)

		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		_ = json.NewEncoder(w).Encode(leases)
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestRead(t *testing.T) {
	srv := serve(t, []Lease{
		{Address: "10.0.0.1", Comment: "host1.lan"},
		{Address: "10.0.0.2", ActiveAddress: "10.0.0.20", Comment: "host2.lan."},
		{Address: "10.0.0.3"},
	})

	d, err := New(slog.Default(), Config{URL: srv.URL, User: "admin", Password: "secret", TTL: 60}, "mikrotik_dhcp.test")
	require.NoError(t, err)
	require.Equal(t, "mikrotik_dhcp.test", d.ID())

	recs, err := d.Read(context.Background())
	require.NoError(t, err)
	require.Equal(t, []dns.Record{
		{Type: dns.A, Name: "host1.lan.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1"), Source: "mikrotik_dhcp.test"},
		{Type: dns.A, Name: "host2.lan.", TTL: 60, Address: netip.MustParseAddr("10.0.0.20"), Source: "mikrotik_dhcp.test"},
	}, recs)
}

func TestReadFilter(t *testing.T) {
	srv := serve(t, []Lease{
		{Address: "10.0.0.1", Comment: "host1.lan"},
		{Address: "192.168.0.1", Comment: "host2.lan"},
	})

	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`filter: {accept: 'subnetContains("10.0.0.0/8", record.address)'}`), &cfg))
	cfg.URL = srv.URL
	cfg.User = "admin"
	cfg.Password = "secret"

	d, err := New(slog.Default(), cfg, "mikrotik_dhcp.test")
	require.NoError(t, err)

	recs, err := d.Read(context.Background())
	require.NoError(t, err)
	require.Len(t, recs, 1)
	require.Equal(t, "host1.lan.", recs[0].Name)
}

func TestReadAuthError(t *testing.T) {
	srv := serve(t, nil)

	d, err := New(slog.Default(), Config{URL: srv.URL, User: "admin", Password: "wrong"}, "mikrotik_dhcp.test")
	require.NoError(t, err)

	_, err = d.Read(context.Background())
	require.ErrorContains(t, err, "401")
}

func TestReadInvalidAddress(t *testing.T) {
	srv := serve(t, []Lease{{Address: "nope", Comment: "host1.lan"}})

	d, err := New(slog.Default(), Config{URL: srv.URL, User: "admin", Password: "secret"}, "mikrotik_dhcp.test")
	require.NoError(t, err)

	_, err = d.Read(context.Background())
	require.Error(t, err)
}
