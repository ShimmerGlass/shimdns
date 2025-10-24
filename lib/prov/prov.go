package prov

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/ShimmerGlass/shimdns/lib/modifier"
	"github.com/ShimmerGlass/shimdns/lib/sink"
	"github.com/ShimmerGlass/shimdns/lib/source"
	"github.com/samber/lo"
)

type Prov struct {
	log *slog.Logger

	interval time.Duration

	sources   []source.Source
	modifiers []modifier.Modifier
	sinks     []sink.Sink

	prev []dns.Record
}

func New(log *slog.Logger, interval time.Duration, sources []source.Source, modifiers []modifier.Modifier, sinks []sink.Sink) (*Prov, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("invalid interval %s", interval)
	}

	return &Prov{
		log:       log,
		interval:  interval,
		sources:   sources,
		modifiers: modifiers,
		sinks:     sinks,
	}, nil
}

func (p *Prov) Run(ctx context.Context) error {
	tick := time.Tick(p.interval)

	for ; ; <-tick {
		err := p.runOnce(ctx)
		if err != nil {
			p.log.Error(err.Error())
		}
	}
}

func (p *Prov) runOnce(ctx context.Context) error {
	p.log.Debug("updating")

	recs, err := p.readRecs(ctx)
	if err != nil {
		return err
	}

	for _, modifier := range p.modifiers {
		recs, err = modifier.Modify(ctx, recs)
		if err != nil {
			return fmt.Errorf("%T: %w", modifier, err)
		}
	}

	err = p.writeRecs(ctx, recs)
	if err != nil {
		return err
	}

	removed, added := lo.Difference(p.prev, recs)
	for _, rec := range removed {
		p.log.Info("removed", "record", rec)
	}

	for _, rec := range added {
		p.log.Info("added", "record", rec)
	}

	p.prev = recs

	return nil
}

func (p *Prov) readRecs(ctx context.Context) ([]dns.Record, error) {
	var lock sync.Mutex
	var recs []dns.Record
	var errs []error

	var wg sync.WaitGroup

	wg.Add(len(p.sources))
	for _, source := range p.sources {
		go func() {
			r, err := source.Read(ctx)

			lock.Lock()
			if err != nil {
				name := source.Type()
				if source.Name() != "" {
					name += "." + source.Name()
				}

				errs = append(errs, fmt.Errorf("%s: %w", name, err))
			} else {
				recs = append(recs, r...)
			}
			lock.Unlock()

			wg.Done()
		}()
	}
	wg.Wait()

	return recs, errors.Join(errs...)
}

func (p *Prov) writeRecs(ctx context.Context, recs []dns.Record) error {
	var lock sync.Mutex
	var errs []error

	var wg sync.WaitGroup

	wg.Add(len(p.sinks))
	for _, sink := range p.sinks {
		go func() {
			err := sink.Write(ctx, recs)

			lock.Lock()
			if err != nil {
				errs = append(errs, fmt.Errorf("%T: %w", sink, err))
			}
			lock.Unlock()

			wg.Done()
		}()
	}
	wg.Wait()

	return errors.Join(errs...)
}
