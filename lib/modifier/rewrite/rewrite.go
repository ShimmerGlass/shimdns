package rewrite

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/ShimmerGlass/shimdns/lib/exp"
)

type Rewrite struct {
	log *slog.Logger
	cfg Config

	rtype      *exp.Prog[string]
	name       *exp.Prog[string]
	address    *exp.Prog[netip.Addr]
	ptr        *exp.Prog[string]
	target     *exp.Prog[string]
	priority   *exp.Prog[int]
	weight     *exp.Prog[int]
	port       *exp.Prog[int]
	preference *exp.Prog[int]
	mx         *exp.Prog[string]
}

func New(log *slog.Logger, cfg Config) (*Rewrite, error) {
	r := &Rewrite{
		log: log.With("modifier", "rewrite"),
		cfg: cfg,
	}

	var err error
	if cfg.Set.Type != "" {
		r.rtype, err = exp.Compile[string](cfg.Set.Type)
		if err != nil {
			return nil, fmt.Errorf("type: %w", err)
		}
	}

	if cfg.Set.Name != "" {
		r.name, err = exp.Compile[string](cfg.Set.Name)
		if err != nil {
			return nil, fmt.Errorf("name: %w", err)
		}
	}

	if cfg.Set.Address != "" {
		r.address, err = exp.Compile[netip.Addr](cfg.Set.Address)
		if err != nil {
			return nil, fmt.Errorf("address: %w", err)
		}
	}

	if cfg.Set.Ptr != "" {
		r.ptr, err = exp.Compile[string](cfg.Set.Ptr)
		if err != nil {
			return nil, fmt.Errorf("ptr: %w", err)
		}
	}

	if cfg.Set.Target != "" {
		r.target, err = exp.Compile[string](cfg.Set.Target)
		if err != nil {
			return nil, fmt.Errorf("target: %w", err)
		}
	}

	if cfg.Set.Priority != "" {
		r.priority, err = exp.Compile[int](cfg.Set.Priority)
		if err != nil {
			return nil, fmt.Errorf("priority: %w", err)
		}
	}

	if cfg.Set.Weight != "" {
		r.weight, err = exp.Compile[int](cfg.Set.Weight)
		if err != nil {
			return nil, fmt.Errorf("weight: %w", err)
		}
	}

	if cfg.Set.Port != "" {
		r.port, err = exp.Compile[int](cfg.Set.Port)
		if err != nil {
			return nil, fmt.Errorf("port: %w", err)
		}
	}

	if cfg.Set.Preference != "" {
		r.preference, err = exp.Compile[int](cfg.Set.Preference)
		if err != nil {
			return nil, fmt.Errorf("preference: %w", err)
		}
	}

	if cfg.Set.Mx != "" {
		r.mx, err = exp.Compile[string](cfg.Set.Mx)
		if err != nil {
			return nil, fmt.Errorf("mx: %w", err)
		}
	}

	return r, nil
}

func (p *Rewrite) Modify(ctx context.Context, records []dns.Record) ([]dns.Record, error) {
	for i, rec := range records {
		ok, err := p.cfg.Filter.Match(rec)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		if p.rtype != nil {
			v, err := p.rtype.Run(rec)
			if err != nil {
				return nil, fmt.Errorf("type: %w", err)
			}
			rec.Type = dns.Type(v)
		}

		if p.name != nil {
			v, err := p.name.Run(rec)
			if err != nil {
				return nil, fmt.Errorf("name: %w", err)
			}
			rec.Name = dns.NormName(v)
		}

		if p.address != nil {
			v, err := p.address.Run(rec)
			if err != nil {
				return nil, fmt.Errorf("address: %w", err)
			}
			rec.Address = v
		}

		if p.ptr != nil {
			v, err := p.ptr.Run(rec)
			if err != nil {
				return nil, fmt.Errorf("ptr: %w", err)
			}
			rec.Ptr = v
		}

		if p.target != nil {
			v, err := p.target.Run(rec)
			if err != nil {
				return nil, fmt.Errorf("target: %w", err)
			}
			rec.Target = v
		}

		if p.priority != nil {
			v, err := p.priority.Run(rec)
			if err != nil {
				return nil, fmt.Errorf("priority: %w", err)
			}
			rec.Priority = uint16(v)
		}

		if p.weight != nil {
			v, err := p.weight.Run(rec)
			if err != nil {
				return nil, fmt.Errorf("weight: %w", err)
			}
			rec.Weight = uint16(v)
		}

		if p.port != nil {
			v, err := p.port.Run(rec)
			if err != nil {
				return nil, fmt.Errorf("port: %w", err)
			}
			rec.Port = uint16(v)
		}

		if p.preference != nil {
			v, err := p.preference.Run(rec)
			if err != nil {
				return nil, fmt.Errorf("preference: %w", err)
			}
			rec.Preference = uint16(v)
		}

		if p.mx != nil {
			v, err := p.mx.Run(rec)
			if err != nil {
				return nil, fmt.Errorf("mx: %w", err)
			}
			rec.Mx = v
		}

		records[i] = rec
	}

	return records, nil
}
