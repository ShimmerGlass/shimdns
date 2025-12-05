package gandi

import (
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
)

func TestRecordHTTPS(t *testing.T) {
	rec := dns.Record{
		Name:     "test.foo.",
		Type:     dns.HTTPS,
		Alpn:     []string{dns.AlpnHTTP2, dns.AlpnHTTP3},
		Priority: 10,
		Target:   ".",
	}
	rvalue := recRValue(rec)

	require.Equal(t, `10 test.foo. (alpn="h2,h3")`, rvalue)

}
