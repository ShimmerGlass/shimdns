package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/modifier/autoptr"
	"github.com/ShimmerGlass/shimdns/lib/modifier/filter"
	"github.com/ShimmerGlass/shimdns/lib/modifier/rewrite"
	"github.com/ShimmerGlass/shimdns/lib/sink/dashboard"
	"github.com/ShimmerGlass/shimdns/lib/sink/dnsserver"
	"github.com/ShimmerGlass/shimdns/lib/sink/gandi"
	httpsink "github.com/ShimmerGlass/shimdns/lib/sink/http"
	"github.com/ShimmerGlass/shimdns/lib/sink/mikrotik"
	"github.com/ShimmerGlass/shimdns/lib/source/file"
	httpsource "github.com/ShimmerGlass/shimdns/lib/source/http"
	mikrotikdhcp "github.com/ShimmerGlass/shimdns/lib/source/mikrotik_dhcp"
	"github.com/ShimmerGlass/shimdns/lib/source/netbox"
	"github.com/ShimmerGlass/shimdns/lib/source/traefik"
	"github.com/stretchr/testify/require"
)

const fullConfig = `
interval: 30s
http_listen_addr: 127.0.0.1:8080

sources:
  - type: file
    name: static
    path: /etc/shimdns/records.yaml
  - type: traefik
    url: http://traefik:8080
    mode: cname
    target: proxy.example.com.
    entrypoints: [websecure]
    ttl: 60
  - type: netbox
    name: nb
    url: http://netbox
    token: secret
    timeout: 5s
  - type: http
    name: remote
    url: http://other/records
    keep_original_source: true
  - type: mikrotik_dhcp
    name: router
    url: http://router
    user: admin
    password: secret
    filter:
      accept: hasSuffix(record.name, ".lan.")

modifiers:
  - type: autoptr
  - type: rewrite
    name: ttl
    set:
      ttl: "300"
  - type: filter
    reject: record.type == "PTR"

sinks:
  - type: dashboard
  - type: mikrotik
    name: router
    url: http://router
    match_comment: true
    comment: shimdns
  - type: dnsserver
    listen_addr: 127.0.0.1:5353
  - type: http
    name: api
    path: /records
  - type: gandi
    domains: [example.com]
    personal_access_token: tok
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestLoadConfig(t *testing.T) {
	cfg, err := loadConfig(writeConfig(t, fullConfig))
	require.NoError(t, err)

	require.Equal(t, 30*time.Second, cfg.Interval)
	require.Equal(t, "127.0.0.1:8080", cfg.HTTPListenAddr)

	require.Len(t, cfg.Sources, 5)
	require.Equal(t, file.Config{Path: "/etc/shimdns/records.yaml"}, cfg.Sources[0].Cfg)
	require.Equal(t, "static", cfg.Sources[0].Name)

	tcfg := cfg.Sources[1].Cfg.(traefik.Config)
	require.Equal(t, "http://traefik:8080", tcfg.URL)
	require.Equal(t, "cname", tcfg.Mode)
	require.Equal(t, "proxy.example.com.", tcfg.Target)
	require.Equal(t, []string{"websecure"}, tcfg.Entrypoints)
	require.Equal(t, 60, tcfg.TTL)

	ncfg := cfg.Sources[2].Cfg.(netbox.Config)
	require.Equal(t, "secret", ncfg.Token)
	require.Equal(t, 5*time.Second, ncfg.Timeout)

	require.True(t, cfg.Sources[3].Cfg.(httpsource.Config).KeepOriginalSource)
	require.Equal(t, "admin", cfg.Sources[4].Cfg.(mikrotikdhcp.Config).User)

	require.Len(t, cfg.Modifiers, 3)
	require.IsType(t, autoptr.Config{}, cfg.Modifiers[0].Cfg)
	require.Equal(t, map[string]string{"ttl": "300"}, cfg.Modifiers[1].Cfg.(rewrite.Config).Set)
	require.IsType(t, filter.Config{}, cfg.Modifiers[2].Cfg)

	require.Len(t, cfg.Sinks, 5)
	require.IsType(t, dashboard.Config{}, cfg.Sinks[0].Cfg)
	mcfg := cfg.Sinks[1].Cfg.(mikrotik.Config)
	require.True(t, mcfg.MatchComment)
	require.Equal(t, "shimdns", mcfg.Comment)
	require.Equal(t, "127.0.0.1:5353", cfg.Sinks[2].Cfg.(dnsserver.Config).ListenAddr)
	require.Equal(t, "/records", cfg.Sinks[3].Cfg.(httpsink.Config).Path)
	gcfg := cfg.Sinks[4].Cfg.(gandi.Config)
	require.Equal(t, []string{"example.com"}, gcfg.Domains)
	require.Equal(t, "tok", gcfg.PersonalAccessToken)
}

func TestLoadConfigErrors(t *testing.T) {
	_, err := loadConfig(filepath.Join(t.TempDir(), "nope.yaml"))
	require.Error(t, err)

	cases := map[string]string{
		"unknown source":   "sources: [{type: nope}]",
		"unknown modifier": "modifiers: [{type: nope}]",
		"unknown sink":     "sinks: [{type: nope}]",
		"bad filter":       "sources: [{type: http, filter: {accept: 'record.nope =='}}]",
		"bad field type":   "sources: [{type: traefik, addresses: [not-an-ip]}]",
		"bad interval":     "interval: often",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfig(writeConfig(t, content))
			require.Error(t, err)
		})
	}
}

func TestLoadSources(t *testing.T) {
	cfg, err := loadConfig(writeConfig(t, fullConfig))
	require.NoError(t, err)

	sources, err := loadSources(slog.Default(), cfg)
	require.NoError(t, err)

	var ids []string
	for _, s := range sources {
		ids = append(ids, s.ID())
	}
	require.Equal(t, []string{
		"file.static",
		"traefik.1",
		"netbox.nb",
		"http.remote",
		"mikrotik_dhcp.router",
	}, ids)
}

func TestLoadModifiers(t *testing.T) {
	cfg, err := loadConfig(writeConfig(t, fullConfig))
	require.NoError(t, err)

	modifiers, err := loadModifiers(slog.Default(), cfg)
	require.NoError(t, err)

	var ids []string
	for _, m := range modifiers {
		ids = append(ids, m.ID())
	}
	require.Equal(t, []string{"autoptr.0", "rewrite.ttl", "filter.2"}, ids)
}

func TestLoadModifiersError(t *testing.T) {
	cfg, err := loadConfig(writeConfig(t, `modifiers: [{type: rewrite, name: bad, set: {nope: "1"}}]`))
	require.NoError(t, err)

	_, err = loadModifiers(slog.Default(), cfg)
	require.ErrorContains(t, err, "rewrite.bad")
}

func TestLoadSinks(t *testing.T) {
	// the dnsserver sink starts a listener on creation, leave it out
	cfg, err := loadConfig(writeConfig(t, `
sinks:
  - type: dashboard
  - type: mikrotik
    name: router
    url: http://router
  - type: http
    name: api
    path: /records
  - type: gandi
    domains: [example.com]
`))
	require.NoError(t, err)

	sinks, err := loadSinks(slog.Default(), cfg, http.NewServeMux())
	require.NoError(t, err)

	var ids []string
	for _, s := range sinks {
		ids = append(ids, s.ID())
	}
	require.Equal(t, []string{"dashboard.0", "mikrotik.router", "http.api", "gandi.3"}, ids)
}

func TestLoadSinksRequireHTTP(t *testing.T) {
	for _, typ := range []string{"dashboard", "http"} {
		cfg, err := loadConfig(writeConfig(t, "sinks: [{type: "+typ+"}]"))
		require.NoError(t, err)

		_, err = loadSinks(slog.Default(), cfg, nil)
		require.ErrorContains(t, err, "http_listen_addr", typ)
	}
}
