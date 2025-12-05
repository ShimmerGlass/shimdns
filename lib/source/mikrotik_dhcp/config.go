package mikrotikdhcp

import (
	"time"

	"github.com/ShimmerGlass/shimdns/lib/exp"
)

type Config struct {
	Name    string        `yaml:"name"`
	Timeout time.Duration `yaml:"timeout"`

	URL      string `yaml:"url"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`

	TTL int `yaml:"ttl"`

	Filter exp.Filter `yaml:"filter"`
}
