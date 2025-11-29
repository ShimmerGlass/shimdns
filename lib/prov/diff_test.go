package prov

import (
	"net/netip"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
)

func TestDiff(t *testing.T) {
	a := dns.Record{Type: dns.A, Address: netip.MustParseAddr("10.1.2.3")}
	b := dns.Record{Type: dns.A, Address: netip.MustParseAddr("10.1.2.4")}
	c := dns.Record{Type: dns.HTTPS, Alpn: []string{dns.AlpnHTTP2}, IPv4Hint: []netip.Addr{netip.MustParseAddr("10.1.2.1")}}
	d := dns.Record{Type: dns.HTTPS, Alpn: []string{dns.AlpnHTTP2, dns.AlpnHTTP3}, IPv4Hint: []netip.Addr{netip.MustParseAddr("10.1.2.1")}}

	added, removed := diff([]dns.Record{a, b, c, d}, []dns.Record{a, b, c, d})
	require.Len(t, added, 0)
	require.Len(t, removed, 0)

	added, removed = diff([]dns.Record{a, b, c}, []dns.Record{a, b, c, d})
	require.ElementsMatch(t, []dns.Record{d}, added)
	require.Len(t, removed, 0)

	added, removed = diff([]dns.Record{a, b, c, d}, []dns.Record{b, c, d})
	require.Len(t, added, 0)
	require.ElementsMatch(t, []dns.Record{a}, removed)

	added, removed = diff([]dns.Record{a, b}, []dns.Record{c, d})
	require.ElementsMatch(t, []dns.Record{c, d}, added)
	require.ElementsMatch(t, []dns.Record{a, b}, removed)
}
