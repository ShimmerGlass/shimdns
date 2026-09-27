package dns

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormName(t *testing.T) {
	require.Equal(t, "foo.bar.", NormName("foo.bar"))
	require.Equal(t, "foo.bar.", NormName("foo.bar."))
	require.Equal(t, "foo.bar.", NormName(".foo.bar"))
	require.Equal(t, "foo.bar.", NormName(".foo.bar."))
}

func TestSubdomainOf(t *testing.T) {
	require.True(t, SubdomainOf("a.example.com.", "example.com."))
	require.True(t, SubdomainOf("a.b.example.com", "example.com"))
	require.True(t, SubdomainOf("a.example.com", "example.com."))

	require.False(t, SubdomainOf("example.com.", "example.com."))
	require.False(t, SubdomainOf("a.example.org.", "example.com."))
	require.False(t, SubdomainOf("aexample.com.", "example.com."))
}

func TestRelativeTo(t *testing.T) {
	require.Equal(t, "a", RelativeTo("a.example.com.", "example.com."))
	require.Equal(t, "a.b", RelativeTo("a.b.example.com", "example.com"))
}

func TestRecordString(t *testing.T) {
	cases := []struct {
		rec  Record
		want string
	}{
		{
			rec:  Record{Type: A, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1")},
			want: "a.example.com. 60 IN A 10.0.0.1",
		},
		{
			rec:  Record{Type: AAAA, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("2001:db8::1")},
			want: "a.example.com. 60 IN AAAA 2001:db8::1",
		},
		{
			rec:  Record{Type: PTR, Name: "1.0.0.10.in-addr.arpa.", TTL: 60, Ptr: "a.example.com."},
			want: "1.0.0.10.in-addr.arpa. 60 IN PTR a.example.com.",
		},
		{
			rec:  Record{Type: CNAME, Name: "b.example.com.", TTL: 60, Target: "a.example.com."},
			want: "b.example.com. 60 IN CNAME a.example.com.",
		},
		{
			rec:  Record{Type: SRV, Name: "_http._tcp.example.com.", TTL: 60, Priority: 10, Weight: 5, Port: 80, Target: "a.example.com."},
			want: "_http._tcp.example.com. 60 IN SRV 10 5 80 a.example.com.",
		},
		{
			rec:  Record{Type: MX, Name: "example.com.", TTL: 60, Preference: 10, Mx: "mail.example.com."},
			want: "example.com. 60 IN MX 10 mail.example.com.",
		},
		{
			rec:  Record{Type: HTTPS, Name: "example.com.", TTL: 60, Priority: 1, Target: "."},
			want: "example.com. 60 IN HTTPS 1 .",
		},
		{
			rec:  Record{Type: HTTPS, Name: "example.com.", TTL: 60, Priority: 1, Target: ".", Alpn: []string{AlpnHTTP2, AlpnHTTP3}},
			want: "example.com. 60 IN HTTPS 1 . alpn=h2,h3",
		},
	}

	for _, tc := range cases {
		t.Run(tc.rec.Type, func(t *testing.T) {
			require.Equal(t, tc.want, tc.rec.String())
		})
	}
}

func TestRecordLogValue(t *testing.T) {
	attrs := func(r Record) map[string]any {
		v := r.LogValue()
		require.Equal(t, slog.KindGroup, v.Kind())

		res := map[string]any{}
		for _, a := range v.Group() {
			res[a.Key] = a.Value.Any()
		}
		return res
	}

	a := attrs(Record{Type: A, Name: "a.example.com.", TTL: 60, Source: "src", Address: netip.MustParseAddr("10.0.0.1")})
	require.Equal(t, "a.example.com.", a["name"])
	require.Equal(t, A, a["type"])
	require.EqualValues(t, 60, a["ttl"])
	require.Equal(t, "src", a["source"])
	require.Equal(t, "10.0.0.1", a["address"])

	srv := attrs(Record{Type: SRV, Target: "t.", Priority: 1, Weight: 2, Port: 3})
	require.Equal(t, "t.", srv["target"])
	require.EqualValues(t, 1, srv["priority"])
	require.EqualValues(t, 2, srv["weight"])
	require.EqualValues(t, 3, srv["port"])

	mx := attrs(Record{Type: MX, Mx: "mail.", Preference: 10})
	require.Equal(t, "mail.", mx["mx"])
	require.EqualValues(t, 10, mx["preference"])

	https := attrs(Record{Type: HTTPS, Target: ".", Priority: 1, Alpn: []string{AlpnHTTP2}})
	require.Equal(t, ".", https["target"])
	require.Equal(t, []string{AlpnHTTP2}, https["alpn"])
}

func TestValidType(t *testing.T) {
	for _, typ := range []string{A, AAAA, PTR, CNAME, SRV, MX, HTTPS} {
		require.True(t, ValidType(typ), typ)
	}

	require.False(t, ValidType("TXT"))
	require.False(t, ValidType(""))
	require.False(t, ValidType("a"))
}

func TestUnknownTypeDoesNotPanic(t *testing.T) {
	r := Record{Type: "TXT", Name: "a.", TTL: 60, Source: "src"}

	require.NotPanics(t, func() {
		_ = r.String()
		_ = r.LogValue()
	})

	require.Len(t, r.LogValue().Group(), 4)
}
