package mikrotik

import "github.com/ShimmerGlass/shimdns/lib/exp"

type Config struct {
	URL      string `yaml:"url"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`

	MatchComment bool   `yaml:"match_comment"`
	Comment      string `yaml:"comment"`

	Filter exp.Filter `yaml:"filter"`
}
