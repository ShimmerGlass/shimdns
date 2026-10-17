package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/sink"
	"github.com/ShimmerGlass/shimdns/lib/sink/dashboard"
	"github.com/ShimmerGlass/shimdns/lib/sink/dnsserver"
	"github.com/ShimmerGlass/shimdns/lib/sink/gandi"
	httpsink "github.com/ShimmerGlass/shimdns/lib/sink/http"
	"github.com/ShimmerGlass/shimdns/lib/sink/mikrotik"
	"gopkg.in/yaml.v3"
)

// DefaultRemovalGrace is how long the gandi sink keeps a record that
// disappeared from the sources. Public resolvers cache the empty answer
// for the zone's SOA minimum (3h at Gandi), so a record that vanishes for
// a few seconds while a container is recreated must not be removed.
const DefaultRemovalGrace = 5 * time.Minute

type SinkConfig struct {
	Type string
	Name string
	Cfg  any

	// RemovalGrace is how long a record that disappeared from the sources
	// is kept in the sink. Zero removes it at once.
	RemovalGrace time.Duration
}

func (s *SinkConfig) UnmarshalYAML(node *yaml.Node) error {
	var cfg typeCfg
	err := node.Decode(&cfg)
	if err != nil {
		return err
	}

	s.Type = cfg.Type
	s.Name = cfg.Name

	// Only gandi defaults to a grace: the others serve or display the
	// records locally, where a dead one does more harm than a missing one.
	grace := struct {
		RemovalGrace *time.Duration `yaml:"removal_grace"`
	}{}
	err = node.Decode(&grace)
	if err != nil {
		return err
	}
	switch {
	case grace.RemovalGrace != nil && *grace.RemovalGrace < 0:
		return fmt.Errorf("sink %q: invalid removal_grace %s", cfg.Type, *grace.RemovalGrace)
	case grace.RemovalGrace != nil:
		s.RemovalGrace = *grace.RemovalGrace
	case cfg.Type == gandi.Type:
		s.RemovalGrace = DefaultRemovalGrace
	}

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

	case gandi.Type:
		rcfg := gandi.Config{}
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
			if httpMux == nil {
				return nil, fmt.Errorf("%s: http_listen_addr must be set", id)
			}
			snk, err = dashboard.New(sinkLog, sinkCfg, id, httpMux)

		case mikrotik.Config:
			snk, err = mikrotik.New(sinkLog, sinkCfg, id)

		case dnsserver.Config:
			snk, err = dnsserver.New(sinkLog, sinkCfg, id)

		case httpsink.Config:
			if httpMux == nil {
				return nil, fmt.Errorf("%s: http_listen_addr must be set", id)
			}
			snk, err = httpsink.New(sinkLog, sinkCfg, id, httpMux)

		case gandi.Config:
			snk, err = gandi.New(sinkLog, sinkCfg, id)

		default:
			return nil, fmt.Errorf("sink %s: unknown type %s", id, anySinkCfg.Type)
		}

		if err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
		sinks = append(sinks, sink.WithRemovalGrace(snk, anySinkCfg.RemovalGrace))
	}

	return sinks, nil
}
