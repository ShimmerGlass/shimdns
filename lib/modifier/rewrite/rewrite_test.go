package rewrite

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	In  dns.Record
	Cfg Config
	Out dns.Record
}

var testCases = []testCase{
	{
		In: dns.Record{
			Type:       dns.A,
			Source:     "src",
			SourceName: "src_name",
			Name:       "foo.bar.",
			Address:    netip.MustParseAddr("127.0.0.1"),
		},
		Cfg: Config{
			Name:    `record.name + "baz."`,
			Address: `ip("192.168.1.1")`,
		},
		Out: dns.Record{
			Type:       dns.A,
			Source:     "src",
			SourceName: "src_name",
			Name:       "foo.bar.baz.",
			Address:    netip.MustParseAddr("192.168.1.1"),
		},
	},
}

func TestRewrite(t *testing.T) {
	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			r, err := New(slog.Default(), tc.Cfg)
			require.NoError(t, err)

			res, err := r.Modify(context.Background(), []dns.Record{tc.In})
			require.NoError(t, err)
			require.Len(t, res, 1)

			require.Equal(t, tc.Out, res[0])
		})
	}
}
