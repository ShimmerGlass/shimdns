package dnsserver

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	dnssrv "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type fakeWriter struct {
	msg *dnssrv.Msg
}

func (f *fakeWriter) LocalAddr() net.Addr          { return &net.UDPAddr{} }
func (f *fakeWriter) RemoteAddr() net.Addr         { return &net.UDPAddr{} }
func (f *fakeWriter) WriteMsg(m *dnssrv.Msg) error { f.msg = m; return nil }
func (f *fakeWriter) Write([]byte) (int, error)    { return 0, nil }
func (f *fakeWriter) Close() error                 { return nil }
func (f *fakeWriter) TsigStatus() error            { return nil }
func (f *fakeWriter) TsigTimersOnly(bool)          {}
func (f *fakeWriter) Hijack()                      {}

// newTestServer builds a server without starting the UDP listener.
func newTestServer(t *testing.T, cfg Config, recs []dns.Record) *DNSServer {
	t.Helper()

	d := &DNSServer{
		log:   slog.Default(),
		cfg:   cfg,
		id:    "dnsserver.test",
		store: &store{},
	}

	require.NoError(t, d.Write(context.Background(), recs))

	return d
}

func query(t *testing.T, d *DNSServer, name string, qtype uint16) *dnssrv.Msg {
	t.Helper()

	req := new(dnssrv.Msg)
	req.SetQuestion(name, qtype)

	w := &fakeWriter{}
	d.handler(w, req)
	require.NotNil(t, w.msg)
	require.Equal(t, req.Id, w.msg.Id)
	require.True(t, w.msg.Authoritative)
	require.True(t, w.msg.Response)

	return w.msg
}

var records = []dns.Record{
	{Type: dns.A, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1")},
	{Type: dns.A, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.2")},
	{Type: dns.AAAA, Name: "a.example.com.", TTL: 120, Address: netip.MustParseAddr("2001:db8::1")},
	{Type: dns.PTR, Name: "1.0.0.10.in-addr.arpa.", TTL: 60, Ptr: "a.example.com."},
	{Type: dns.SRV, Name: "_http._tcp.example.com.", TTL: 60, Priority: 10, Weight: 5, Port: 8080, Target: "a.example.com."},
	{Type: dns.MX, Name: "example.com.", TTL: 60, Preference: 10, Mx: "mail.example.com."},
	{Type: dns.HTTPS, Name: "a.example.com.", TTL: 60, Priority: 1, Target: ".", Alpn: []string{dns.AlpnHTTP2, dns.AlpnHTTP3}},
	{Type: dns.HTTPS, Name: "b.example.com.", TTL: 60, Priority: 1, Target: "a.example.com."},
	{Type: dns.CNAME, Name: "c.example.com.", TTL: 90, Target: "a.example.com."},
}

func TestA(t *testing.T) {
	d := newTestServer(t, Config{}, records)

	res := query(t, d, "a.example.com.", dnssrv.TypeA)
	require.Len(t, res.Answer, 2)

	var got []string
	for _, rr := range res.Answer {
		a, ok := rr.(*dnssrv.A)
		require.True(t, ok)
		require.Equal(t, "a.example.com.", a.Hdr.Name)
		require.EqualValues(t, 60, a.Hdr.Ttl)
		require.Equal(t, uint16(dnssrv.ClassINET), a.Hdr.Class)
		got = append(got, a.A.String())
	}
	require.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2"}, got)
}

func TestAAAA(t *testing.T) {
	d := newTestServer(t, Config{}, records)

	res := query(t, d, "a.example.com.", dnssrv.TypeAAAA)
	require.Len(t, res.Answer, 1)

	aaaa := res.Answer[0].(*dnssrv.AAAA)
	require.Equal(t, "2001:db8::1", aaaa.AAAA.String())
	require.EqualValues(t, 120, aaaa.Hdr.Ttl)
}

func TestPTR(t *testing.T) {
	d := newTestServer(t, Config{}, records)

	res := query(t, d, "1.0.0.10.in-addr.arpa.", dnssrv.TypePTR)
	require.Len(t, res.Answer, 1)
	require.Equal(t, "a.example.com.", res.Answer[0].(*dnssrv.PTR).Ptr)
}

func TestSRV(t *testing.T) {
	d := newTestServer(t, Config{}, records)

	res := query(t, d, "_http._tcp.example.com.", dnssrv.TypeSRV)
	require.Len(t, res.Answer, 1)

	srv := res.Answer[0].(*dnssrv.SRV)
	require.EqualValues(t, 10, srv.Priority)
	require.EqualValues(t, 5, srv.Weight)
	require.EqualValues(t, 8080, srv.Port)
	require.Equal(t, "a.example.com.", srv.Target)
}

func TestMX(t *testing.T) {
	d := newTestServer(t, Config{}, records)

	res := query(t, d, "example.com.", dnssrv.TypeMX)
	require.Len(t, res.Answer, 1)

	mx := res.Answer[0].(*dnssrv.MX)
	require.EqualValues(t, 10, mx.Preference)
	require.Equal(t, "mail.example.com.", mx.Mx)
}

func extraAddrs(msg *dnssrv.Msg) []string {
	var res []string
	for _, rr := range msg.Extra {
		switch rr := rr.(type) {
		case *dnssrv.A:
			res = append(res, rr.A.String())
		case *dnssrv.AAAA:
			res = append(res, rr.AAAA.String())
		}
	}
	return res
}

func TestHTTPSSelfTarget(t *testing.T) {
	d := newTestServer(t, Config{}, records)

	res := query(t, d, "a.example.com.", dnssrv.TypeHTTPS)
	require.Len(t, res.Answer, 1)

	https := res.Answer[0].(*dnssrv.HTTPS)
	require.EqualValues(t, 1, https.Priority)
	require.Equal(t, ".", https.Target)
	require.Len(t, https.Value, 1)
	require.Equal(t, []string{dns.AlpnHTTP2, dns.AlpnHTTP3}, https.Value[0].(*dnssrv.SVCBAlpn).Alpn)

	require.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2", "2001:db8::1"}, extraAddrs(res))
}

func TestHTTPSOtherTarget(t *testing.T) {
	d := newTestServer(t, Config{}, records)

	res := query(t, d, "b.example.com.", dnssrv.TypeHTTPS)
	require.Len(t, res.Answer, 1)

	https := res.Answer[0].(*dnssrv.HTTPS)
	require.Equal(t, "a.example.com.", https.Target)
	require.Empty(t, https.Value)

	require.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2", "2001:db8::1"}, extraAddrs(res))
}

func TestCNAME(t *testing.T) {
	d := newTestServer(t, Config{}, records)

	res := query(t, d, "c.example.com.", dnssrv.TypeA)
	require.Equal(t, dnssrv.RcodeSuccess, res.Rcode)
	require.Len(t, res.Answer, 3)

	cname, ok := res.Answer[0].(*dnssrv.CNAME)
	require.True(t, ok)
	require.Equal(t, "c.example.com.", cname.Hdr.Name)
	require.EqualValues(t, 90, cname.Hdr.Ttl)
	require.Equal(t, "a.example.com.", cname.Target)

	var addrs []string
	for _, rr := range res.Answer[1:] {
		a := rr.(*dnssrv.A)
		require.Equal(t, "a.example.com.", a.Hdr.Name)
		addrs = append(addrs, a.A.String())
	}
	require.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2"}, addrs)

	res = query(t, d, "c.example.com.", dnssrv.TypeAAAA)
	require.Len(t, res.Answer, 2)
	require.IsType(t, &dnssrv.CNAME{}, res.Answer[0])
	require.Equal(t, "2001:db8::1", res.Answer[1].(*dnssrv.AAAA).AAAA.String())

	// other types follow the CNAME too
	res = query(t, d, "c.example.com.", dnssrv.TypeHTTPS)
	require.Len(t, res.Answer, 2)
	require.IsType(t, &dnssrv.CNAME{}, res.Answer[0])
	require.Equal(t, "a.example.com.", res.Answer[1].(*dnssrv.HTTPS).Hdr.Name)
}

func TestCNAMEQuery(t *testing.T) {
	d := newTestServer(t, Config{}, records)

	res := query(t, d, "c.example.com.", dnssrv.TypeCNAME)
	require.Len(t, res.Answer, 1)
	require.Equal(t, "a.example.com.", res.Answer[0].(*dnssrv.CNAME).Target)

	// a name without CNAME answers NODATA
	res = query(t, d, "a.example.com.", dnssrv.TypeCNAME)
	require.Equal(t, dnssrv.RcodeSuccess, res.Rcode)
	require.Empty(t, res.Answer)
}

func TestCNAMEChain(t *testing.T) {
	d := newTestServer(t, Config{}, []dns.Record{
		{Type: dns.CNAME, Name: "x.example.com.", TTL: 60, Target: "y.example.com."},
		{Type: dns.CNAME, Name: "y.example.com.", TTL: 60, Target: "z.example.com."},
		{Type: dns.A, Name: "z.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1")},
	})

	res := query(t, d, "x.example.com.", dnssrv.TypeA)
	require.Len(t, res.Answer, 3)
	require.Equal(t, "y.example.com.", res.Answer[0].(*dnssrv.CNAME).Target)
	require.Equal(t, "z.example.com.", res.Answer[1].(*dnssrv.CNAME).Target)
	require.Equal(t, "10.0.0.1", res.Answer[2].(*dnssrv.A).A.String())
}

func TestCNAMELoop(t *testing.T) {
	d := newTestServer(t, Config{}, []dns.Record{
		{Type: dns.CNAME, Name: "x.example.com.", TTL: 60, Target: "y.example.com."},
		{Type: dns.CNAME, Name: "y.example.com.", TTL: 60, Target: "x.example.com."},
	})

	res := query(t, d, "x.example.com.", dnssrv.TypeA)
	require.Len(t, res.Answer, maxCNAMEChain)
}

func TestCNAMEExternalTarget(t *testing.T) {
	d := newTestServer(t, Config{}, []dns.Record{
		{Type: dns.CNAME, Name: "x.example.com.", TTL: 60, Target: "elsewhere.example.org."},
	})

	res := query(t, d, "x.example.com.", dnssrv.TypeA)
	require.Equal(t, dnssrv.RcodeSuccess, res.Rcode)
	require.Len(t, res.Answer, 1)
	require.Equal(t, "elsewhere.example.org.", res.Answer[0].(*dnssrv.CNAME).Target)
}

func TestUnknownName(t *testing.T) {
	d := newTestServer(t, Config{}, records)

	for _, qtype := range []uint16{dnssrv.TypeA, dnssrv.TypeAAAA, dnssrv.TypePTR, dnssrv.TypeSRV, dnssrv.TypeMX, dnssrv.TypeHTTPS, dnssrv.TypeCNAME} {
		res := query(t, d, "nope.example.com.", qtype)
		require.Equal(t, dnssrv.RcodeNameError, res.Rcode)
		require.Empty(t, res.Answer)
	}
}

func TestNoData(t *testing.T) {
	d := newTestServer(t, Config{}, records)

	res := query(t, d, "example.com.", dnssrv.TypeA)
	require.Equal(t, dnssrv.RcodeSuccess, res.Rcode)
	require.Empty(t, res.Answer)
}

func TestEmptyNonTerminal(t *testing.T) {
	d := newTestServer(t, Config{}, []dns.Record{
		{Type: dns.A, Name: "traefik.vault.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1")},
		{Type: dns.PTR, Name: "10.15.168.192.in-addr.arpa.", TTL: 60, Ptr: "a.example.com."},
	})

	for _, name := range []string{"vault.example.com.", "example.com.", "com.", "15.168.192.in-addr.arpa.", "168.192.in-addr.arpa."} {
		res := query(t, d, name, dnssrv.TypeA)
		require.Equal(t, dnssrv.RcodeSuccess, res.Rcode, name)
		require.Empty(t, res.Answer, name)
	}

	for _, name := range []string{"other.vault.example.com.", "x.traefik.vault.example.com.", "11.15.168.192.in-addr.arpa."} {
		res := query(t, d, name, dnssrv.TypeA)
		require.Equal(t, dnssrv.RcodeNameError, res.Rcode, name)
	}
}

func TestCaseInsensitive(t *testing.T) {
	d := newTestServer(t, Config{}, []dns.Record{
		{Type: dns.A, Name: "Mixed.Example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1")},
	})

	for _, name := range []string{"mixed.example.com.", "MIXED.EXAMPLE.COM.", "Mixed.Example.com."} {
		res := query(t, d, name, dnssrv.TypeA)
		require.Len(t, res.Answer, 1, name)
		// the answer keeps the case of the question
		require.Equal(t, name, res.Answer[0].Header().Name)
	}
}

func TestWriteReplaces(t *testing.T) {
	d := newTestServer(t, Config{}, records)

	require.NoError(t, d.Write(context.Background(), []dns.Record{
		{Type: dns.A, Name: "new.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.9")},
	}))

	require.Empty(t, query(t, d, "a.example.com.", dnssrv.TypeA).Answer)
	require.Len(t, query(t, d, "new.example.com.", dnssrv.TypeA).Answer, 1)
}

func TestWriteFilter(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`filter: {reject: 'record.type == "AAAA"'}`), &cfg))

	d := newTestServer(t, cfg, records)

	require.Len(t, query(t, d, "a.example.com.", dnssrv.TypeA).Answer, 2)
	require.Empty(t, query(t, d, "a.example.com.", dnssrv.TypeAAAA).Answer)
}

func TestWriteFilterError(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`filter: {accept: 'record.address == ip("bad")'}`), &cfg))

	d := &DNSServer{log: slog.Default(), cfg: cfg, store: &store{}}
	require.Error(t, d.Write(context.Background(), records))
}

func TestStore(t *testing.T) {
	s := &store{}
	s.reset()

	require.Nil(t, s.get("a.", dns.A))

	s.add(dns.Record{Type: dns.A, Name: "a."})
	s.add(dns.Record{Type: dns.A, Name: "a."})
	s.add(dns.Record{Type: dns.AAAA, Name: "a."})

	require.Len(t, s.get("a.", dns.A), 2)
	require.Len(t, s.get("a.", dns.AAAA), 1)
	require.Nil(t, s.get("a.", dns.MX))
	require.Nil(t, s.get("b.", dns.A))

	require.True(t, s.has("a."))
	require.True(t, s.has("A."))
	require.False(t, s.has("b."))

	s.add(dns.Record{Type: dns.A, Name: "x.y.z."})
	require.True(t, s.has("y.z."))
	require.True(t, s.has("z."))
	require.Nil(t, s.get("y.z.", dns.A))
}
