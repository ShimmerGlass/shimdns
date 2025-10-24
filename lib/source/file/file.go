package http

import (
	"context"
	"log/slog"
	"os"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/samber/lo"
	"gopkg.in/yaml.v3"
)

const Type = "file"

type File struct {
	log *slog.Logger
	cfg Config
}

func New(log *slog.Logger, cfg Config) (*File, error) {
	if cfg.Name == "" {
		cfg.Name = cfg.Path
	}

	return &File{
		log: log.With("source", Type, "source_name", cfg.Name),
		cfg: cfg,
	}, nil
}

func (f *File) Type() string {
	return Type
}

func (f *File) Name() string {
	return f.cfg.Name
}

func (f *File) Read(ctx context.Context) ([]dns.Record, error) {
	file, err := os.Open(f.cfg.Path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	d := dns.Records{}
	err = yaml.NewDecoder(file).Decode(&d)
	if err != nil {
		return nil, err
	}

	recs := lo.Map(d.Records, func(rec dns.Record, _ int) dns.Record {
		if rec.Source == "" {
			rec.Source = Type
		}

		if rec.SourceName == "" {
			rec.SourceName = f.cfg.Name
		}

		return rec
	})

	return recs, nil
}
