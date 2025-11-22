package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ShimmerGlass/shimdns/lib/sink"
	"github.com/ShimmerGlass/shimdns/lib/sink/dashboard"
	"github.com/ShimmerGlass/shimdns/lib/sink/dnsserver"
	"github.com/ShimmerGlass/shimdns/lib/sink/gandi"
	httpsink "github.com/ShimmerGlass/shimdns/lib/sink/http"
	"github.com/ShimmerGlass/shimdns/lib/sink/mikrotik"
	"gopkg.in/yaml.v3"
)

type SinkConfig struct {
	Type string
	Name string
	Cfg  any
}

func (s *SinkConfig) UnmarshalYAML(node *yaml.Node) error {
	var cfg typeCfg
	err := node.Decode(&cfg)
	if err != nil {
		return err
	}

	s.Type = cfg.Type
	s.Name = cfg.Name

	switch cfg.Type {
	case dashboard.Type:
		rcfg := dashboard.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case mikrotik.Type:
		rcfg := mikrotik.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case dnsserver.Type:
		rcfg := dnsserver.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case httpsink.Type:
		rcfg := httpsink.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	default:
		return fmt.Errorf("unknown sink type %q", cfg.Type)
	}

	return nil
}

func loadSinks(log *slog.Logger, cfg Config, httpMux *http.ServeMux) ([]sink.Sink, error) {
	sinks := []sink.Sink{}

	for i, anySinkCfg := range cfg.Sinks {
		name := anySinkCfg.Name
		if name == "" {
			name = strconv.Itoa(i)
		}
		id := fmt.Sprintf("%s.%s", anySinkCfg.Type, name)

		sinkLog := log.With("sink", id)

		var snk sink.Sink
		var err error

		switch sinkCfg := anySinkCfg.Cfg.(type) {

		case dashboard.Config:
			snk, err = dashboard.New(sinkLog, sinkCfg, id, httpMux)

		case mikrotik.Config:
			snk, err = mikrotik.New(sinkLog, sinkCfg, id)

		case dnsserver.Config:
			snk, err = dnsserver.New(sinkLog, sinkCfg, id)

		case httpsink.Config:
			snk, err = httpsink.New(sinkLog, sinkCfg, id, httpMux)

		default:
			return nil, fmt.Errorf("sink %s: unknown type %s", id, anySinkCfg.Type)
		}

		if err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
		sinks = append(sinks, snk)
	}

	return sinks, nil
}
