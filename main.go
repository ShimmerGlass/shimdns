package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/ShimmerGlass/shimdns/lib/prov"
)

func main() {
	err := run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	cfgPath := flag.String("c", "config.yaml", "config file path")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	ctx := context.Background()

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	var httpMux *http.ServeMux
	if cfg.HTTPListenAddr != "" {
		httpMux = http.NewServeMux()
	}

	sources, err := loadSources(log, cfg)
	if err != nil {
		return err
	}

	modifiers, err := loadModifiers(log, cfg)
	if err != nil {
		return err
	}

	sinks, err := loadSinks(log, cfg, httpMux)
	if err != nil {
		return err
	}

	prov, err := prov.New(log, cfg.Interval, sources, modifiers, sinks)
	if err != nil {
		return err
	}

	if httpMux != nil {
		go func() {
			log.Info("http: listening", "addr", cfg.HTTPListenAddr)
			err := http.ListenAndServe(cfg.HTTPListenAddr, httpMux)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}()
	}

	return prov.Run(ctx)
}
