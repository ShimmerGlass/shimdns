package dnsserver

import "github.com/ShimmerGlass/shimdns/lib/exp"

type Config struct {
	ListenAddr string `yaml:"listen_addr"`

	// Upstream is the resolver (host:port) asked for the records of a CNAME
	// target that is not in the store. Without it, such answers hold only the
	// CNAME chain.
	Upstream string `yaml:"upstream"`

	Filter exp.Filter `yaml:"filter"`
}
