package gandi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

	require.Equal(t, `10 test.foo. alpn="h2,h3"`, rvalue)

}

func TestRecordRValue(t *testing.T) {
	require.Equal(t, "1 target.foo.", recRValue(dns.Record{Type: dns.HTTPS, Name: "test.foo.", Priority: 1, Target: "target.foo."}))
	require.Equal(t, "10.0.0.1", recRValue(dns.Record{Type: dns.A, Address: netip.MustParseAddr("10.0.0.1")}))
	require.Equal(t, "a.foo.", recRValue(dns.Record{Type: dns.CNAME, Target: "a.foo."}))
	require.Equal(t, "10 mail.foo.", recRValue(dns.Record{Type: dns.MX, Preference: 10, Mx: "mail.foo."}))
}

func sortedByName(recs []*DomainRecord) map[recKey]*DomainRecord {
	res := map[recKey]*DomainRecord{}
	for _, r := range recs {
		res[recKey{Name: r.RrsetName, Type: r.RrsetType}] = r
	}
	return res
}

var testRecords = []dns.Record{
	{Type: dns.A, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1")},
	{Type: dns.A, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.2")},
	{Type: dns.AAAA, Name: "a.example.com.", TTL: 3600, Address: netip.MustParseAddr("2001:db8::1")},
	{Type: dns.CNAME, Name: "b.sub.example.com.", TTL: 600, Target: "a.example.com."},
	{Type: dns.A, Name: "a.example.org.", TTL: 60, Address: netip.MustParseAddr("10.0.0.3")},
	{Type: dns.A, Name: "a.notexample.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.4")},
}

func TestBuildDomain(t *testing.T) {
	g, err := New(slog.Default(), Config{}, "gandi.test")
	require.NoError(t, err)

	recs := sortedByName(g.buildDomain("example.com", testRecords))

	require.Equal(t, map[recKey]*DomainRecord{
		{Name: "a", Type: dns.A}: {
			RrsetType:   dns.A,
			RrsetName:   "a",
			RrsetTTL:    300,
			RrsetValues: []string{"10.0.0.1", "10.0.0.2"},
		},
		{Name: "a", Type: dns.AAAA}: {
			RrsetType:   dns.AAAA,
			RrsetName:   "a",
			RrsetTTL:    3600,
			RrsetValues: []string{"2001:db8::1"},
		},
		{Name: "b.sub", Type: dns.CNAME}: {
			RrsetType:   dns.CNAME,
			RrsetName:   "b.sub",
			RrsetTTL:    600,
			RrsetValues: []string{"a.example.com."},
		},
	}, recs)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type capturedReq struct {
	method string
	url    string
	auth   string
	ctype  string
	body   Records
}

func newTestGandi(t *testing.T, cfg Config, status int) (*Gandi, *[]capturedReq) {
	t.Helper()

	g, err := New(slog.Default(), cfg, "gandi.test")
	require.NoError(t, err)

	reqs := &[]capturedReq{}
	g.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		c := capturedReq{
			method: r.Method,
			url:    r.URL.String(),
			auth:   r.Header.Get("Authorization"),
			ctype:  r.Header.Get("Content-Type"),
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&c.body))
		*reqs = append(*reqs, c)

		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     http.Header{},
		}, nil
	})

	return g, reqs
}

func TestWrite(t *testing.T) {
	g, reqs := newTestGandi(t, Config{
		PersonalAccessToken: "tok",
		Domains:             []string{"example.com", "example.org"},
	}, http.StatusCreated)

	require.NoError(t, g.Write(context.Background(), testRecords))
	require.Len(t, *reqs, 2)

	com := (*reqs)[0]
	require.Equal(t, http.MethodPut, com.method)
	require.Equal(t, "https://api.gandi.net/v5/livedns/domains/example.com/records", com.url)
	require.Equal(t, "Bearer tok", com.auth)
	require.Equal(t, "application/json", com.ctype)
	require.Len(t, com.body.Items, 3)

	org := (*reqs)[1]
	require.Equal(t, "https://api.gandi.net/v5/livedns/domains/example.org/records", org.url)
	require.Equal(t, []*DomainRecord{{
		RrsetType:   dns.A,
		RrsetName:   "a",
		RrsetTTL:    300,
		RrsetValues: []string{"10.0.0.3"},
	}}, org.body.Items)
}

func TestWriteFilter(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`filter: {accept: 'record.type == "CNAME"'}`), &cfg))
	cfg.Domains = []string{"example.com"}

	g, reqs := newTestGandi(t, cfg, http.StatusOK)

	require.NoError(t, g.Write(context.Background(), testRecords))
	require.Len(t, *reqs, 1)
	require.Len(t, (*reqs)[0].body.Items, 1)
	require.Equal(t, dns.CNAME, (*reqs)[0].body.Items[0].RrsetType)
}

func TestWriteError(t *testing.T) {
	g, reqs := newTestGandi(t, Config{Domains: []string{"example.com", "example.org"}}, http.StatusForbidden)

	err := g.Write(context.Background(), testRecords)
	require.ErrorContains(t, err, "403")
	require.Len(t, *reqs, 1)
}

func TestBuildDomainApex(t *testing.T) {
	g, err := New(slog.Default(), Config{}, "gandi.test")
	require.NoError(t, err)

	recs := g.buildDomain("example.com", []dns.Record{
		{Type: dns.MX, Name: "example.com.", TTL: 300, Preference: 10, Mx: "mail.example.com."},
		{Type: dns.MX, Name: "example.com.", TTL: 300, Preference: 20, Mx: "mail2.example.com."},
	})

	require.Equal(t, []*DomainRecord{{
		RrsetType:   dns.MX,
		RrsetName:   "@",
		RrsetTTL:    300,
		RrsetValues: []string{"10 mail.example.com.", "20 mail2.example.com."},
	}}, recs)
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}

func TestWriteClosesBody(t *testing.T) {
	g, err := New(slog.Default(), Config{Domains: []string{"example.com", "example.org"}}, "gandi.test")
	require.NoError(t, err)

	var bodies []*trackedBody
	g.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b := &trackedBody{Reader: strings.NewReader("{}")}
		bodies = append(bodies, b)
		return &http.Response{StatusCode: http.StatusOK, Body: b, Header: http.Header{}}, nil
	})

	require.NoError(t, g.Write(context.Background(), testRecords))
	require.Len(t, bodies, 2)
	for _, b := range bodies {
		require.True(t, b.closed)
	}
}

func TestWriteContext(t *testing.T) {
	g, err := New(slog.Default(), Config{Domains: []string{"example.com"}}, "gandi.test")
	require.NoError(t, err)

	g.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = g.Write(ctx, testRecords)
	require.ErrorIs(t, err, context.Canceled)
}
