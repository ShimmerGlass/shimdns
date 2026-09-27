package file

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/ShimmerGlass/shimdns/lib/dns"
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

	for i, rec := range d.Records {
		if !dns.ValidType(rec.Type) {
			return nil, fmt.Errorf("record %d (%s): unsupported type %q", i, rec.Name, rec.Type)
		}

		if rec.Source == "" {
			d.Records[i].Source = f.id
		}
	}

	return d.Records, nil
}
