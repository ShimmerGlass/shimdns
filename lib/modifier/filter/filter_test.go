package filter

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFilter(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`
accept: hasSuffix(record.name, ".example.com.")
reject: record.type == "PTR"
`), &cfg))

	f, err := New(slog.Default(), cfg, "filter.test")
	require.NoError(t, err)
	require.Equal(t, "filter.test", f.ID())

	in := []dns.Record{
		{Type: dns.A, Name: "a.example.com."},
		{Type: dns.A, Name: "a.example.org."},
		{Type: dns.PTR, Name: "a.example.com."},
		{Type: dns.CNAME, Name: "b.example.com."},
	}

	out, err := f.Modify(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, []dns.Record{in[0], in[3]}, out)
}

func TestFilterEmpty(t *testing.T) {
	f, err := New(slog.Default(), Config{}, "filter.test")
	require.NoError(t, err)

	in := []dns.Record{{Type: dns.A, Name: "a."}, {Type: dns.A, Name: "b."}}

	out, err := f.Modify(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, in, out)
}

func TestFilterError(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`accept: 'record.address == ip("bad")'`), &cfg))

	f, err := New(slog.Default(), cfg, "filter.test")
	require.NoError(t, err)

	_, err = f.Modify(context.Background(), []dns.Record{{Type: dns.A}})
	require.Error(t, err)
}
