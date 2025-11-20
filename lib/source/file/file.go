package file

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
	id  string
}

func New(log *slog.Logger, cfg Config, id string) (*File, error) {
	return &File{
		log: log,
		cfg: cfg,
		id:  id,
	}, nil
}

func (f *File) ID() string {
	return f.id
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
			rec.Source = f.id
		}

		return rec
	})

	return recs, nil
}
