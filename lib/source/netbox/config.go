package netbox

import (
	"time"

	"github.com/ShimmerGlass/shimdns/lib/exp"
)

type Config struct {
	Name    string        `yaml:"name"`
	URL     string        `yaml:"url"`
	Token   string        `yaml:"token"`
	Timeout time.Duration `yaml:"timeout"`
	Filter  exp.Filter    `yaml:"filter"`
}
