package main

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/ShimmerGlass/shimdns/lib/source"
	"github.com/ShimmerGlass/shimdns/lib/source/file"
	httpsource "github.com/ShimmerGlass/shimdns/lib/source/http"
	mikrotikdhcp "github.com/ShimmerGlass/shimdns/lib/source/mikrotik_dhcp"
	"github.com/ShimmerGlass/shimdns/lib/source/netbox"
	"github.com/ShimmerGlass/shimdns/lib/source/traefik"
	"gopkg.in/yaml.v3"
)

type SourceConfig struct {
	Type string
	Name string
	Cfg  any
}

func (s *SourceConfig) UnmarshalYAML(node *yaml.Node) error {
	var cfg typeCfg
	err := node.Decode(&cfg)
	if err != nil {
		return err
	}

	s.Type = cfg.Type
	s.Name = cfg.Name

	switch cfg.Type {

	case traefik.Type:
		rcfg := traefik.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case netbox.Type:
		rcfg := netbox.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case httpsource.Type:
		rcfg := httpsource.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case mikrotikdhcp.Type:
		rcfg := mikrotikdhcp.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	default:
		return fmt.Errorf("unknown source type %q", cfg.Type)
	}

	return nil
}

func loadSources(log *slog.Logger, cfg Config) ([]source.Source, error) {
	sources := []source.Source{}

	for i, anySrcCfg := range cfg.Sources {
		name := anySrcCfg.Name
		if name == "" {
			name = strconv.Itoa(i)
		}
		id := fmt.Sprintf("%s.%s", anySrcCfg.Type, name)

		srcLog := log.With("source", id)

		var src source.Source
		var err error

		switch srcCfg := anySrcCfg.Cfg.(type) {

		case file.Config:
			src, err = file.New(srcLog, srcCfg, id)

		case traefik.Config:
			src, err = traefik.New(srcLog, srcCfg, id)

		case netbox.Config:
			src, err = netbox.New(srcLog, srcCfg, id)

		case httpsource.Config:
			src, err = httpsource.New(srcLog, srcCfg, id)

		case mikrotikdhcp.Config:
			src, err = mikrotikdhcp.New(srcLog, srcCfg, id)

		default:
			return nil, fmt.Errorf("source %s: unknown type %q", id, anySrcCfg.Type)
		}

		if err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
		sources = append(sources, src)
	}

	return sources, nil
}
