package traefik

import (
	"net/netip"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/exp"
)

const (
	modeAddress = "address"
	modeCname   = "cname"
)

type Config struct {
	Name        string        `yaml:"name"`
	URL         string        `yaml:"url"`
	Timeout     time.Duration `yaml:"timeout"`
	Entrypoints []string      `yaml:"entrypoints"`

	Mode      string       `yaml:"mode"`
	Target    string       `yaml:"target"`
	Addresses []netip.Addr `yaml:"addresses"`

	Filter exp.Filter `yaml:"filter"`
}
