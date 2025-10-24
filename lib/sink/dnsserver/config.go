package dnsserver

import "github.com/ShimmerGlass/shimdns/lib/exp"

type Config struct {
	ListenAddr string `yaml:"listen_addr"`

	Filter exp.Filter `yaml:"filter"`
}
