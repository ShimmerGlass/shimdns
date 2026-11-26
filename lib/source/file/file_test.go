package file

import (
	"context"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

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
  - type: TXT
    name: example.com.
    ttl: 300
    text: v=spf1 include:_mailcust.gandi.net ?all
  - type: ALIAS
    name: example.com.
    ttl: 300
    target: site.scw.cloud.
  - type: CAA
    name: example.com.
    ttl: 300
    flags: 128
    tag: issue
    value: letsencrypt.org
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
		{Type: dns.TXT, Name: "example.com.", TTL: 300, Text: "v=spf1 include:_mailcust.gandi.net ?all", Source: "file.test"},
		{Type: dns.ALIAS, Name: "example.com.", TTL: 300, Target: "site.scw.cloud.", Source: "file.test"},
		{Type: dns.CAA, Name: "example.com.", TTL: 300, Flags: 128, Tag: "issue", Value: "letsencrypt.org", Source: "file.test"},
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
	require.NoError(t, os.WriteFile(path, []byte(`records: [{type: NS, name: a.example.com.}]`), 0o600))

	f, err := New(slog.Default(), Config{Path: path}, "file.test")
	require.NoError(t, err)

	_, err = f.Read(context.Background())
	require.ErrorContains(t, err, "NS")
}

func waitChange(t *testing.T, ch <-chan struct{}) bool {
	t.Helper()
	select {
	case <-ch:
		return true
	case <-time.After(2 * time.Second):
		return false
	}
}

func drain(ch <-chan struct{}) {
	for {
		select {
		case <-ch:
		case <-time.After(200 * time.Millisecond):
			return
		}
	}
}

func TestChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.yaml")
	require.NoError(t, os.WriteFile(path, []byte("records: []"), 0o600))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := New(slog.Default(), Config{Path: path}, "file.test")
	require.NoError(t, err)
	ch := f.Changes(ctx)
	require.NotNil(t, ch)

	t.Run("in place", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte("records: [] # 1"), 0o600))
		require.True(t, waitChange(t, ch))
	})
	drain(ch)

	t.Run("rename into place", func(t *testing.T) {
		tmp := filepath.Join(dir, "records.yaml.tmp")
		require.NoError(t, os.WriteFile(tmp, []byte("records: [] # 2"), 0o600))
		drain(ch)
		require.NoError(t, os.Rename(tmp, path))
		require.True(t, waitChange(t, ch))
	})
	drain(ch)

	t.Run("unrelated file", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "other.yaml"), []byte("x"), 0o600))
		require.False(t, waitChange(t, ch))
	})
}

func TestChangesDisabled(t *testing.T) {
	no := false
	f, err := New(slog.Default(), Config{Path: filepath.Join(t.TempDir(), "a.yaml"), Watch: &no}, "file.test")
	require.NoError(t, err)
	require.Nil(t, f.Changes(context.Background()))
}

func TestChangesMissingDir(t *testing.T) {
	f, err := New(slog.Default(), Config{Path: filepath.Join(t.TempDir(), "nodir", "a.yaml")}, "file.test")
	require.NoError(t, err)
	require.Nil(t, f.Changes(context.Background()))
}
