package rewrite

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type testCase struct {
	In  dns.Record
	Cfg Config
	Out dns.Record
}

var testCases = []testCase{
	{
		In: dns.Record{
			Type:    dns.A,
			Source:  "src",
			Name:    "foo.bar.",
			Address: netip.MustParseAddr("127.0.0.1"),
		},
		Cfg: Config{
			Set: map[string]string{
				"name":    `record.name + "baz."`,
				"address": `ip("192.168.1.1")`,
			},
		},
		Out: dns.Record{
			Type:    dns.A,
			Source:  "src",
			Name:    "foo.bar.baz.",
			Address: netip.MustParseAddr("192.168.1.1"),
		},
	},
	{
		In: dns.Record{
			Type:   dns.HTTPS,
			Source: "src",
			Name:   "foo.bar.",
			Alpn:   []string{dns.AlpnHTTP2},
		},
		Cfg: Config{
			Set: map[string]string{
				"alpn": `["h2", "h3"]`,
			},
		},
		Out: dns.Record{
			Type:   dns.HTTPS,
			Source: "src",
			Name:   "foo.bar.",
			Alpn:   []string{dns.AlpnHTTP2, dns.AlpnHTTP3},
		},
	},
}

func TestRewrite(t *testing.T) {
	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			r, err := New(slog.Default(), tc.Cfg, "test")
			require.NoError(t, err)

			res, err := r.Modify(context.Background(), []dns.Record{tc.In})
			require.NoError(t, err)
			require.Len(t, res, 1)

			require.Equal(t, tc.Out, res[0])
		})
	}
}

func TestRewriteFilter(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`
filter:
  accept: record.type == "A"
set:
  ttl: "300"
`), &cfg))

	r, err := New(slog.Default(), cfg, "test")
	require.NoError(t, err)

	in := []dns.Record{
		{Type: dns.A, Name: "a.", TTL: 60},
		{Type: dns.CNAME, Name: "b.", TTL: 60, Target: "a."},
	}

	res, err := r.Modify(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, []dns.Record{
		{Type: dns.A, Name: "a.", TTL: 300},
		{Type: dns.CNAME, Name: "b.", TTL: 60, Target: "a."},
	}, res)
}

func TestRewriteInvalidField(t *testing.T) {
	_, err := New(slog.Default(), Config{Set: map[string]string{"nope": `"x"`}}, "test")
	require.ErrorContains(t, err, "nope")
}

func TestRewriteInvalidExpr(t *testing.T) {
	_, err := New(slog.Default(), Config{Set: map[string]string{"name": `record.name +`}}, "test")
	require.ErrorContains(t, err, "set.name")
}

func TestRewriteRuntimeError(t *testing.T) {
	r, err := New(slog.Default(), Config{Set: map[string]string{"address": `ip("bad")`}}, "test")
	require.NoError(t, err)

	_, err = r.Modify(context.Background(), []dns.Record{{Type: dns.A}})
	require.Error(t, err)
}

func TestRewriteUsesOriginalRecord(t *testing.T) {
	r, err := New(slog.Default(), Config{Set: map[string]string{
		"name":   `"new."`,
		"ptr":    `record.name`,
		"target": `record.name + "t."`,
		"mx":     `record.name + "m."`,
	}}, "test")
	require.NoError(t, err)

	// map iteration order is random, run several times to catch ordering issues
	for range 50 {
		res, err := r.Modify(context.Background(), []dns.Record{{Type: dns.PTR, Name: "old."}})
		require.NoError(t, err)
		require.Equal(t, dns.Record{Type: dns.PTR, Name: "new.", Ptr: "old.", Target: "old.t.", Mx: "old.m."}, res[0])
	}
}

func TestRewriteTypeMismatch(t *testing.T) {
	cases := map[string]string{
		"address": `"10.0.0.1"`,
		"ttl":     `1.5`,
		"name":    `42`,
		"alpn":    `[1, 2]`,
	}

	for field, exp := range cases {
		t.Run(field, func(t *testing.T) {
			r, err := New(slog.Default(), Config{Set: map[string]string{field: exp}}, "test")
			require.NoError(t, err)

			_, err = r.Modify(context.Background(), []dns.Record{{Type: dns.A, Name: "a."}})
			require.ErrorContains(t, err, "cannot assign")
		})
	}
}

func TestRewriteNil(t *testing.T) {
	r, err := New(slog.Default(), Config{Set: map[string]string{"port": `nil`}}, "test")
	require.NoError(t, err)

	_, err = r.Modify(context.Background(), []dns.Record{{Type: dns.A, Name: "a."}})
	require.Error(t, err)
}
