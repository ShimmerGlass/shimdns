package main

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/ShimmerGlass/shimdns/lib/modifier"
	"github.com/ShimmerGlass/shimdns/lib/modifier/autoptr"
	"github.com/ShimmerGlass/shimdns/lib/modifier/filter"
	"github.com/ShimmerGlass/shimdns/lib/modifier/rewrite"
	"gopkg.in/yaml.v3"
)

type ModifierConfig struct {
	Type string
	Name string
	Cfg  any
}

func (s *ModifierConfig) UnmarshalYAML(node *yaml.Node) error {
	var cfg typeCfg
	err := node.Decode(&cfg)
	if err != nil {
		return err
	}

	s.Type = cfg.Type
	s.Name = cfg.Name

	switch cfg.Type {

	case autoptr.Type:
		rcfg := autoptr.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case rewrite.Type:
		rcfg := rewrite.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case filter.Type:
		rcfg := filter.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	default:
		return fmt.Errorf("unknown modifier type %q", cfg.Type)
	}

	return nil
}

func loadModifiers(log *slog.Logger, cfg Config) ([]modifier.Modifier, error) {
	modifiers := []modifier.Modifier{}

	for i, anyModCfg := range cfg.Modifiers {
		name := anyModCfg.Name
		if name == "" {
			name = strconv.Itoa(i)
		}
		id := fmt.Sprintf("%s.%s", anyModCfg.Type, name)

		modLog := log.With("modifier", id)

		var mod modifier.Modifier
		var err error

		switch modCfg := anyModCfg.Cfg.(type) {
		case autoptr.Config:
			mod, err = autoptr.New(modLog, modCfg, id)

		case rewrite.Config:
			mod, err = rewrite.New(modLog, modCfg, id)

		case filter.Config:
			mod, err = filter.New(modLog, modCfg, id)

		default:
			return nil, fmt.Errorf("modifier %s: unknown type %s", id, anyModCfg.Type)

		}

		if err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}

		modifiers = append(modifiers, mod)
	}

	return modifiers, nil
}
