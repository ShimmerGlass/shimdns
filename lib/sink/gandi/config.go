package gandi

import (
	"time"

	"github.com/ShimmerGlass/shimdns/lib/exp"
)

type Config struct {
	PersonalAccessToken string        `yaml:"personal_access_token"`
	Timeout             time.Duration `yaml:"timeout"`
	Domains             []string      `yaml:"domains"`

	Filter exp.Filter `yaml:"filter"`
}
