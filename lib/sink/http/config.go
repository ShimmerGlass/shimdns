package http

import "github.com/ShimmerGlass/shimdns/lib/exp"

type Config struct {
	Path string `yaml:"path"`

	Filter exp.Filter `yaml:"filter"`
}
