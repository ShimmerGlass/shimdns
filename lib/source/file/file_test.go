package file

import (
	"context"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
records:
  - type: A
    name: a.example.com.
    ttl: 60
    address: 10.0.0.1
  - type: CNAME
    name: b.example.com.
    ttl: 60
    target: a.example.com.
    source: custom
  - type: HTTPS
    name: a.example.com.
    priority: 1
    target: .
    alpn: [h2, h3]
`), 0o600))

	f, err := New(slog.Default(), Config{Path: path}, "file.test")
	require.NoError(t, err)
	require.Equal(t, "file.test", f.ID())

	recs, err := f.Read(context.Background())
	require.NoError(t, err)
	require.Equal(t, []dns.Record{
		{Type: dns.A, Name: "a.example.com.", TTL: 60, Address: netip.MustParseAddr("10.0.0.1"), Source: "file.test"},
		{Type: dns.CNAME, Name: "b.example.com.", TTL: 60, Target: "a.example.com.", Source: "custom"},
		{Type: dns.HTTPS, Name: "a.example.com.", Priority: 1, Target: ".", Alpn: []string{"h2", "h3"}, Source: "file.test"},
	}, recs)
}

func TestReadMissingFile(t *testing.T) {
	f, err := New(slog.Default(), Config{Path: filepath.Join(t.TempDir(), "nope.yaml")}, "file.test")
	require.NoError(t, err)

	_, err = f.Read(context.Background())
	require.Error(t, err)
}

func TestReadInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`records: [{address: not-an-ip}]`), 0o600))

	f, err := New(slog.Default(), Config{Path: path}, "file.test")
	require.NoError(t, err)

	_, err = f.Read(context.Background())
	require.Error(t, err)
}

func TestReadUnsupportedType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`records: [{type: TXT, name: a.example.com.}]`), 0o600))

	f, err := New(slog.Default(), Config{Path: path}, "file.test")
	require.NoError(t, err)

	_, err = f.Read(context.Background())
	require.ErrorContains(t, err, "TXT")
}
