package autoptr

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net/netip"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAddrToPTR(t *testing.T) {
	ptr, err := addrToPTR(netip.MustParseAddr("192.168.1.10"))
	require.NoError(t, err)
	require.Equal(t, "10.1.168.192.in-addr.arpa.", ptr)

	ptr, err = addrToPTR(netip.MustParseAddr("2001:db8::567:89ab"))
	require.NoError(t, err)
	require.Equal(t, "b.a.9.8.7.6.5.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.", ptr)

	_, err = addrToPTR(netip.Addr{})
	require.Error(t, err)
}

func TestPTRToAddr(t *testing.T) {
	addr, err := ptrToAddr("10.1.168.192.in-addr.arpa.")
	require.NoError(t, err)
	require.Equal(t, netip.MustParseAddr("192.168.1.10"), addr)

	addr, err = ptrToAddr("B.A.9.8.7.6.5.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.B.D.0.1.0.0.2.IP6.ARPA.")
	require.NoError(t, err)
	require.Equal(t, netip.MustParseAddr("2001:db8::567:89ab"), addr)

	for _, bad := range []string{
		"foo.example.com.",
		"1.168.192.in-addr.arpa.",
		"1.0.ip6.arpa.",
		"x.1.168.192.in-addr.arpa.",
	} {
		_, err := ptrToAddr(bad)
		require.Error(t, err, bad)
	}
}

func TestPTRRoundTrip(t *testing.T) {
	for _, s := range []string{"0.0.0.0", "10.0.0.1", "255.255.255.255", "::1", "fe80::1", "2001:db8:85a3::8a2e:370:7334"} {
		addr := netip.MustParseAddr(s)

		ptr, err := addrToPTR(addr)
		require.NoError(t, err)

		back, err := ptrToAddr(ptr)
		require.NoError(t, err)
		require.Equal(t, addr, back, s)
	}
}

func TestModify(t *testing.T) {
	p, err := New(slog.Default(), Config{}, "autoptr.test")
	require.NoError(t, err)

	in := []dns.Record{
		{Type: dns.A, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1"), Source: "src"},
		{Type: dns.AAAA, Name: "a.example.com.", TTL: 120, Address: netip.MustParseAddr("2001:db8::1"), Source: "src"},
		// same address as the first record: only one PTR must be generated
		{Type: dns.A, Name: "b.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1"), Source: "src"},
		// already has an explicit PTR
		{Type: dns.A, Name: "c.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.3"), Source: "src"},
		{Type: dns.PTR, Name: "3.0.0.10.in-addr.arpa.", TTL: 60, Ptr: "explicit.example.com.", Source: "src"},
		{Type: dns.CNAME, Name: "d.example.com.", TTL: 60, Target: "a.example.com.", Source: "src"},
	}

	out, err := p.Modify(context.Background(), append([]dns.Record{}, in...))
	require.NoError(t, err)

	require.Equal(t, in, out[:len(in)])
	require.ElementsMatch(t, []dns.Record{
		{Type: dns.PTR, Name: "1.0.0.10.in-addr.arpa.", TTL: 60, Ptr: "a.example.com.", Source: "autoptr.test"},
		{Type: dns.PTR, Name: "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.", TTL: 120, Ptr: "a.example.com.", Source: "autoptr.test"},
	}, out[len(in):])
}

func TestModifyStable(t *testing.T) {
	p, err := New(slog.Default(), Config{}, "autoptr.test")
	require.NoError(t, err)

	addr := netip.MustParseAddr("82.66.74.86")
	in := []dns.Record{
		{Type: dns.A, Name: "c.example.com.", TTL: 60, Address: addr},
		{Type: dns.A, Name: "a.example.com.", TTL: 120, Address: addr},
		{Type: dns.A, Name: "b.example.com.", TTL: 60, Address: addr},
		{Type: dns.A, Name: "d.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1")},
		{Type: dns.A, Name: "e.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.2")},
	}

	want := []dns.Record{
		{Type: dns.PTR, Name: "1.0.0.10.in-addr.arpa.", TTL: 60, Ptr: "d.example.com.", Source: "autoptr.test"},
		{Type: dns.PTR, Name: "2.0.0.10.in-addr.arpa.", TTL: 60, Ptr: "e.example.com.", Source: "autoptr.test"},
		{Type: dns.PTR, Name: "86.74.66.82.in-addr.arpa.", TTL: 120, Ptr: "a.example.com.", Source: "autoptr.test"},
	}

	for i := range 50 {
		shuffled := append([]dns.Record{}, in...)
		rand.New(rand.NewPCG(uint64(i), 0)).Shuffle(len(shuffled), func(a, b int) {
			shuffled[a], shuffled[b] = shuffled[b], shuffled[a]
		})

		out, err := p.Modify(context.Background(), shuffled)
		require.NoError(t, err)
		require.Equal(t, want, out[len(in):])
	}
}

func TestModifyFilter(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`filter: {accept: 'record.name == "a.example.com."'}`), &cfg))

	p, err := New(slog.Default(), cfg, "autoptr.test")
	require.NoError(t, err)

	in := []dns.Record{
		{Type: dns.A, Name: "a.example.com.", Address: netip.MustParseAddr("10.0.0.1")},
		{Type: dns.A, Name: "b.example.com.", Address: netip.MustParseAddr("10.0.0.2")},
	}

	out, err := p.Modify(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, out, 3)
	require.Equal(t, "1.0.0.10.in-addr.arpa.", out[2].Name)
}

func TestModifyInvalidPTR(t *testing.T) {
	p, err := New(slog.Default(), Config{}, "autoptr.test")
	require.NoError(t, err)

	_, err = p.Modify(context.Background(), []dns.Record{
		{Type: dns.PTR, Name: "not-a-ptr.example.com.", Ptr: "a.example.com."},
	})
	require.Error(t, err)
}
