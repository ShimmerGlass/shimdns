package netbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type nbAddr struct {
	address string
	dnsName string
}

func ipAddressJSON(id int, a nbAddr) map[string]any {
	return map[string]any{
		"id":          id,
		"url":         fmt.Sprintf("http://netbox/api/ipam/ip-addresses/%d/", id),
		"display":     a.address,
		"family":      map[string]any{"value": 4, "label": "IPv4"},
		"address":     a.address,
		"nat_outside": []any{},
		"dns_name":    a.dnsName,
	}
}

// serve emulates the netbox ip-addresses list endpoint, returning pageSize
// results per page.
func serve(t *testing.T, addrs []nbAddr, pageSize int) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/ipam/ip-addresses/", r.URL.Path)
		require.Equal(t, "Token secret", r.Header.Get("Authorization"))
		require.Equal(t, "false", r.URL.Query().Get("dns_name__empty"))

		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		results := []any{}
		for i := offset; i < len(addrs) && i < offset+pageSize; i++ {
			results = append(results, ipAddressJSON(i+1, addrs[i]))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"count":   len(addrs),
			"results": results,
		})
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestRead(t *testing.T) {
	srv := serve(t, []nbAddr{
		{address: "10.0.0.1/24", dnsName: "a.example.com"},
		{address: "10.0.0.2/24", dnsName: "b.example.com."},
		{address: "2001:db8::1/64", dnsName: "c.example.com"},
		{address: "not-an-ip", dnsName: "d.example.com"},
		{address: "10.0.0.5/32", dnsName: "e.example.com"},
	}, 2)

	n, err := New(slog.Default(), Config{URL: srv.URL, Token: "secret", TTL: 60}, "netbox.test")
	require.NoError(t, err)
	require.Equal(t, "netbox.test", n.ID())

	recs, err := n.Read(context.Background())
	require.NoError(t, err)
	require.Equal(t, []dns.Record{
		{Type: dns.A, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1"), Source: "netbox.test"},
		{Type: dns.A, Name: "b.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.2"), Source: "netbox.test"},
		{Type: dns.AAAA, Name: "c.example.com.", TTL: 60, Address: netip.MustParseAddr("2001:db8::1"), Source: "netbox.test"},
		{Type: dns.A, Name: "e.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.5"), Source: "netbox.test"},
	}, recs)
}

func TestReadFilter(t *testing.T) {
	srv := serve(t, []nbAddr{
		{address: "10.0.0.1/24", dnsName: "a.example.com"},
		{address: "2001:db8::1/64", dnsName: "c.example.com"},
	}, 50)

	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`filter: {accept: 'record.type == "AAAA"'}`), &cfg))
	cfg.URL = srv.URL
	cfg.Token = "secret"

	n, err := New(slog.Default(), cfg, "netbox.test")
	require.NoError(t, err)

	recs, err := n.Read(context.Background())
	require.NoError(t, err)
	require.Len(t, recs, 1)
	require.Equal(t, "c.example.com.", recs[0].Name)
}

func TestReadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	n, err := New(slog.Default(), Config{URL: srv.URL, Token: "secret"}, "netbox.test")
	require.NoError(t, err)

	_, err = n.Read(context.Background())
	require.Error(t, err)
}
