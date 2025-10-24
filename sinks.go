package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ShimmerGlass/shimdns/lib/sink"
	"github.com/ShimmerGlass/shimdns/lib/sink/dashboard"
	"github.com/ShimmerGlass/shimdns/lib/sink/dnsserver"
	httpsink "github.com/ShimmerGlass/shimdns/lib/sink/http"
	"github.com/ShimmerGlass/shimdns/lib/sink/mikrotik"
	"gopkg.in/yaml.v3"
)

const (
	sinkDashboard = "dashboard"
	sinkMikrotik  = "mikrotik"
	sinkDNSServer = "dnsserver"
	sinkHTTP      = "http"
)

type SinkConfig struct {
	Type string
	Cfg  any
}

func (s *SinkConfig) UnmarshalYAML(node *yaml.Node) error {
	var cfg typeCfg
	err := node.Decode(&cfg)
	if err != nil {
		return err
	}

	switch cfg.Type {
	case sinkDashboard:
		rcfg := dashboard.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case sinkMikrotik:
		rcfg := mikrotik.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case sinkDNSServer:
		rcfg := dnsserver.Config{}
		err = node.Decode(&rcfg)
		if err != nil {
			return err
		}
		s.Cfg = rcfg

	case sinkHTTP:
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

	for _, anySinkCfg := range cfg.Sinks {
		switch sinkCfg := anySinkCfg.Cfg.(type) {

		case dashboard.Config:
			src, err := dashboard.New(log, sinkCfg, httpMux)
			if err != nil {
				return nil, fmt.Errorf("dashboard: %w", err)
			}

			sinks = append(sinks, src)

		case mikrotik.Config:
			src, err := mikrotik.New(log, sinkCfg)
			if err != nil {
				return nil, fmt.Errorf("mikrotik: %w", err)
			}

			sinks = append(sinks, src)

		case dnsserver.Config:
			src, err := dnsserver.New(log, sinkCfg)
			if err != nil {
				return nil, fmt.Errorf("dnsserver: %w", err)
			}

			sinks = append(sinks, src)

		case httpsink.Config:
			src, err := httpsink.New(log, sinkCfg, httpMux)
			if err != nil {
				return nil, fmt.Errorf("dnsserver: %w", err)
			}

			sinks = append(sinks, src)

		default:
			return nil, fmt.Errorf("unknown sink type %s", anySinkCfg.Type)
		}
	}

	return sinks, nil
}
