package mikrotik

import "github.com/ShimmerGlass/shimdns/lib/exp"

const (
	defaultTTL = "1d"
)

type Config struct {
	URL      string `yaml:"url"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`

	MatchComment bool   `yaml:"match_comment"`
	Comment      string `yaml:"comment"`
	TTL          string `yaml:"ttl"`

	Filter exp.Filter `yaml:"filter"`
}
