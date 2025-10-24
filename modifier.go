package main

import (
	"fmt"
	"log/slog"

	"github.com/ShimmerGlass/shimdns/lib/modifier"
	"github.com/ShimmerGlass/shimdns/lib/modifier/autoptr"
	"github.com/ShimmerGlass/shimdns/lib/modifier/filter"
	"github.com/ShimmerGlass/shimdns/lib/modifier/rewrite"
	"gopkg.in/yaml.v3"
)

const (
	modifierAutoPTR = "autoptr"
	modifierRewrite = "rewrite"
	modifierFilter  = "filter"
)

type ModifierConfig struct {
	Type string
	Cfg  any
}

func (s *ModifierConfig) UnmarshalYAML(node *yaml.Node) error {
	var cfg typeCfg
	err := node.Decode(&cfg)
	if err != nil {
		return err
	}

	switch cfg.Type {

	case modifierAutoPTR:
		rcfg := autoptr.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case modifierRewrite:
		rcfg := rewrite.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case modifierFilter:
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

	for _, anyProcCfg := range cfg.Modifiers {
		switch modCfg := anyProcCfg.Cfg.(type) {

		case autoptr.Config:
			modifier, err := autoptr.New(log, modCfg)
			if err != nil {
				return nil, fmt.Errorf("autoptr: %w", err)
			}

			modifiers = append(modifiers, modifier)

		case rewrite.Config:
			modifier, err := rewrite.New(log, modCfg)
			if err != nil {
				return nil, fmt.Errorf("rewrite: %w", err)
			}

			modifiers = append(modifiers, modifier)

		case filter.Config:
			modifier, err := filter.New(log, modCfg)
			if err != nil {
				return nil, fmt.Errorf("filter: %w", err)
			}

			modifiers = append(modifiers, modifier)

		default:
			return nil, fmt.Errorf("unknown modifier type %s", anyProcCfg.Type)
		}
	}

	return modifiers, nil
}
