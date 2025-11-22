package rewrite

import (
	"github.com/ShimmerGlass/shimdns/lib/exp"
)

type Config struct {
	Filter exp.Filter        `yaml:"filter"`
	Set    map[string]string `yaml:"set"`
}
