package main

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Interval       time.Duration `yaml:"interval"`
	HTTPListenAddr string        `yaml:"http_listen_addr"`

	Sources   []SourceConfig   `yaml:"sources"`
	Modifiers []ModifierConfig `yaml:"modifiers"`
	Sinks     []SinkConfig     `yaml:"sinks"`
}

func loadConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer func() { _ = f.Close() }()

	var cfg Config

	err = yaml.NewDecoder(f).Decode(&cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

type typeCfg struct {
	Type string `yaml:"type"`
	Name string `yaml:"name"`
}
