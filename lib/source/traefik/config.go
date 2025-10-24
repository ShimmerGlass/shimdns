package traefik

import (
	"time"

	"github.com/ShimmerGlass/shimdns/lib/exp"
)

type Config struct {
	Name        string        `yaml:"name"`
	URL         string        `yaml:"url"`
	Timeout     time.Duration `yaml:"timeout"`
	Entrypoints []string      `yaml:"entrypoints"`
	Filter      exp.Filter    `yaml:"filter"`
}
