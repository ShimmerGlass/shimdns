package http

import (
	"time"

	"github.com/ShimmerGlass/shimdns/lib/exp"
)

type Config struct {
	Name               string        `yaml:"name"`
	URL                string        `yaml:"url"`
	Timeout            time.Duration `yaml:"timeout"`
	KeepOriginalSource bool          `yaml:"keep_original_source"`
	Filter             exp.Filter    `yaml:"filter"`
}
