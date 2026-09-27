package mikrotik

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/samber/lo"
)

const Type = "mikrotik"

type Mikrotik struct {
	cfg Config
	log *slog.Logger
	id  string

	api *api
}

func New(log *slog.Logger, cfg Config, id string) (*Mikrotik, error) {
	return &Mikrotik{
		cfg: cfg,
		log: log,
		id:  id,
		api: newAPI(cfg.URL, cfg.User, cfg.Password),
	}, nil
}

func (m *Mikrotik) ID() string {
	return m.id
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

	records, err = m.supportedRecords(records)
	if err != nil {
		return err
	}

	toAdd := []dns.Record{}
	toRemove := []entry{}

	for _, rec := range records {
		found := false

		for _, e := range current {
			if m.entryMatchesRecord(e, rec) {
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
			if m.entryMatchesRecord(e, rec) {
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
		e := m.recordToEntry(rec)

		m.log.Info("adding entry", "entry", e)

		err = m.api.Add(ctx, e)
		if err != nil {
			return err
		}
	}

	return nil
}

// supportedRecords returns the records accepted by the filter that can be
// represented as mikrotik static dns entries.
func (m *Mikrotik) supportedRecords(records []dns.Record) ([]dns.Record, error) {
	res := []dns.Record{}

	for _, rec := range records {
		ok, err := m.cfg.Filter.Match(rec)
		if err != nil {
			return nil, err
		}

		if !ok {
			continue
		}

		switch rec.Type {
		case dns.A, dns.AAAA:
			res = append(res, rec)

		default:
			m.log.Debug("record type not supported", "record", rec)
		}
	}

	return res, nil
}

func (m *Mikrotik) recordToEntry(rec dns.Record) entry {
	// TODO: strip end dot

	return entry{
		Type:     rec.Type,
		Name:     rec.Name,
		Address:  rec.Address.String(),
		Comment:  m.cfg.Comment,
		TTL:      strconv.Itoa(int(rec.TTL)),
		Disabled: "false",
	}
}

func (m *Mikrotik) entryMatchesRecord(e entry, rec dns.Record) bool {
	if e.Comment != m.cfg.Comment {
		return false
	}

	return e.Name+"." == rec.Name && e.Address == rec.Address.String()
}
