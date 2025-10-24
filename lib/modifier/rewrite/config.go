package rewrite

import (
	"github.com/ShimmerGlass/shimdns/lib/exp"
)

type Config struct {
	Filter exp.Filter `yaml:"filter"`

	RecordType string `yaml:"record_type"`
	Name       string `yaml:"name"`

	// for A & AAAA
	Address string `yaml:"address"`
	// for PTR
	Ptr string `yaml:"ptr"`
	// for CNAME & SRV
	Target string `yaml:"target"`
	// for SRV
	Priority string `yaml:"priority"`
	Weight   string `yaml:"weight"`
	Port     string `yaml:"port"`
	// for MX
	Preference string `yaml:"preference"`
	Mx         string `yaml:"mx"`
}
