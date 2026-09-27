package exp

import (
	"net/netip"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func parseFilter(t *testing.T, src string) Filter {
	t.Helper()

	var f Filter
	require.NoError(t, yaml.Unmarshal([]byte(src), &f))
	return f
}

func TestReject(t *testing.T) {
	r, err := NewReject(`record.type == "A"`)
	require.NoError(t, err)

	ok, err := r.Match(dns.Record{Type: dns.A})
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = r.Match(dns.Record{Type: dns.AAAA})
	require.NoError(t, err)
	require.True(t, ok)
}

func TestFilterEmptyMatchesAll(t *testing.T) {
	var f Filter

	ok, err := f.Match(dns.Record{Type: dns.A})
	require.NoError(t, err)
	require.True(t, ok)
}

func TestFilterAcceptReject(t *testing.T) {
	f := parseFilter(t, `
accept: hasSuffix(record.name, ".example.com.")
reject: record.type == "PTR"
`)

	cases := []struct {
		rec  dns.Record
		want bool
	}{
		{dns.Record{Type: dns.A, Name: "a.example.com."}, true},
		{dns.Record{Type: dns.A, Name: "a.example.org."}, false},
		{dns.Record{Type: dns.PTR, Name: "a.example.com."}, false},
		{dns.Record{Type: dns.PTR, Name: "a.example.org."}, false},
	}

	for _, tc := range cases {
		ok, err := f.Match(tc.rec)
		require.NoError(t, err)
		require.Equal(t, tc.want, ok, "%s %s", tc.rec.Type, tc.rec.Name)
	}
}

func TestFilterFilter(t *testing.T) {
	f := parseFilter(t, `accept: subnetContains("10.0.0.0/8", record.address)`)

	in := []dns.Record{
		{Name: "a.", Address: netip.MustParseAddr("10.0.0.1")},
		{Name: "b.", Address: netip.MustParseAddr("192.168.0.1")},
		{Name: "c.", Address: netip.MustParseAddr("10.1.2.3")},
	}

	out, err := f.Filter(in)
	require.NoError(t, err)
	require.Equal(t, []dns.Record{in[0], in[2]}, out)
}

func TestFilterUnmarshalErrors(t *testing.T) {
	var f Filter
	err := yaml.Unmarshal([]byte(`accept: "record.nope =="`), &f)
	require.ErrorContains(t, err, "accept")

	f = Filter{}
	err = yaml.Unmarshal([]byte(`reject: "record.nope =="`), &f)
	require.ErrorContains(t, err, "reject")
}

func TestCompileWrongReturnType(t *testing.T) {
	_, err := NewAccept(`record.name`)
	require.Error(t, err)

	_, err = NewAccept(`record.unknown_field == "x"`)
	require.Error(t, err)
}

func TestRuntimeErrors(t *testing.T) {
	acc, err := NewAccept(`subnetContains("not a subnet", record.address)`)
	require.NoError(t, err)

	_, err = acc.Match(dns.Record{Address: netip.MustParseAddr("10.0.0.1")})
	require.Error(t, err)

	acc, err = NewAccept(`record.address == ip("not an ip")`)
	require.NoError(t, err)

	_, err = acc.Match(dns.Record{})
	require.Error(t, err)
}

func TestCompileAny(t *testing.T) {
	p, err := CompileAny(`record.ttl * 2`)
	require.NoError(t, err)

	v, err := p.Run(dns.Record{TTL: 30})
	require.NoError(t, err)
	require.Equal(t, 60, v)

	p, err = CompileAny(`subnet("10.0.0.0/8")`)
	require.NoError(t, err)

	v, err = p.Run(dns.Record{})
	require.NoError(t, err)
	require.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), v)
}
