package mikrotik

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/samber/lo"
)

type Mikrotik struct {
	cfg Config
	log *slog.Logger

	api *api
}

func New(log *slog.Logger, cfg Config) (*Mikrotik, error) {
	if cfg.TTL == "" {
		cfg.TTL = defaultTTL
	}

	// TODO: validate config

	return &Mikrotik{
		cfg: cfg,
		log: log.With("sink", "mikrotik"),
		api: newAPI(cfg.URL, cfg.User, cfg.Password),
	}, nil
}

func (m *Mikrotik) Write(ctx context.Context, records []dns.Record) error {
	err := m.write(ctx, records)
	if err != nil {
		return fmt.Errorf("mikrotik sink: %w", err)
	}

	return nil
}

func (m *Mikrotik) write(ctx context.Context, records []dns.Record) error {
	current, err := m.api.Entries(ctx)
	if err != nil {
		return fmt.Errorf("mikrotik: %w", err)
	}

	if m.cfg.MatchComment {
		current = lo.Filter(current, func(e entry, _ int) bool {
			return e.Comment == m.cfg.Comment
		})
	}

	toAdd := []dns.Record{}
	toRemove := []entry{}

	for _, rec := range records {
		found := false

		for _, e := range current {
			match, err := m.entryMatchesRecord(e, rec)
			if err != nil {
				return err
			}

			if match {
				found = true
				break
			}
		}

		if !found {
			toAdd = append(toAdd, rec)
		}
	}

	for _, e := range current {
		found := false

		for _, rec := range records {
			match, err := m.entryMatchesRecord(e, rec)
			if err != nil {
				return err
			}

			if match {
				found = true
				break
			}
		}

		if !found {
			toRemove = append(toRemove, e)
		}
	}

	for _, e := range toRemove {
		m.log.Info("removing entry", "entry", e)

		err := m.api.Delete(ctx, e.ID)
		if err != nil {
			return err
		}
	}

	for _, rec := range toAdd {
		e, ok, err := m.recordToEntry(rec)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		m.log.Info("adding entry", "entry", e)

		err = m.api.Add(ctx, e)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Mikrotik) recordToEntry(rec dns.Record) (entry, bool, error) {
	// TODO: strip end dot

	switch rec.Type {
	case dns.A:
		return entry{
			Type:     "A",
			Name:     rec.Name,
			Address:  rec.Address.String(),
			Comment:  m.cfg.Comment,
			TTL:      "1d",
			Disabled: "false",
		}, true, nil

	case dns.AAAA:
		return entry{
			Type:     "AAAA",
			Name:     rec.Name,
			Address:  rec.Address.String(),
			Comment:  m.cfg.Comment,
			TTL:      m.cfg.TTL,
			Disabled: "false",
		}, true, nil

	case dns.PTR:
		// mikrotik static dns entries do not support PTR records
		return entry{}, false, nil

	default:
		return entry{}, false, fmt.Errorf("record type %T not handled", rec)
	}
}

func (m *Mikrotik) entryMatchesRecord(e entry, rec dns.Record) (bool, error) {
	if e.Comment != m.cfg.Comment {
		return false, nil
	}

	switch rec.Type {

	case dns.A, dns.AAAA:
		return e.Name+"." == rec.Name && e.Address == rec.Address.String(), nil

	case dns.PTR:
		return false, nil

	default:
		return false, fmt.Errorf("record type %T not handled", rec)
	}
}
