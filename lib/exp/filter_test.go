package exp

import (
	"net/netip"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	Record dns.Record
	Expr   string
	Result bool
}

var testCases = []testCase{
	{
		Record: dns.Record{Name: "foo.bar."},
		Expr:   `hasSuffix(record.name, ".bar.")`,
		Result: true,
	},
	{
		Record: dns.Record{Name: "foo.bar."},
		Expr:   `hasSuffix(record.name, ".boo.")`,
		Result: false,
	},
	{
		Record: dns.Record{Address: netip.MustParseAddr("192.168.1.10")},
		Expr:   `record.address == ip("192.168.1.10")`,
		Result: true,
	},
	{
		Record: dns.Record{Address: netip.MustParseAddr("192.168.1.10")},
		Expr:   `record.address != ip("127.0.0.1")`,
		Result: true,
	},
	{
		Record: dns.Record{Address: netip.MustParseAddr("192.168.1.10")},
		Expr:   `subnetContains("192.168.1.0/24", record.address)`,
		Result: true,
	},
	{
		Record: dns.Record{Address: netip.MustParseAddr("192.168.1.10")},
		Expr:   `subnetContains("192.168.2.0/24", record.address)`,
		Result: false,
	},
	{
		Record: dns.Record{Address: netip.MustParseAddr("192.168.1.10")},
		Expr:   `subnetContains(subnet("192.168.1.0/24"), record.address)`,
		Result: true,
	},
	{
		Record: dns.Record{},
		Expr:   `subnetContains(subnet("192.168.1.0/24"), "192.168.1.24")`,
		Result: true,
	},
}

func TestAccept(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.Expr, func(t *testing.T) {
			acc, err := NewAccept(tc.Expr)
			require.NoError(t, err)

			ok, err := acc.Match(tc.Record)
			require.NoError(t, err)

			require.Equal(t, tc.Result, ok)
		})
	}
}
