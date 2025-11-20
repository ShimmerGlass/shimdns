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

	for _, mod := range p.modifiers {
		recs, err = p.applyModifier(ctx, mod, recs)
		if err != nil {
			return err
		}
	}

	old := lo.Map(p.prev, func(r dns.Record, _ int) string { return r.String() })
	new := lo.Map(recs, func(r dns.Record, _ int) string { return r.String() })

	removed, added := lo.Difference(old, new)
	for _, rec := range removed {
		p.log.Info("removed", "record", rec)
	}

	for _, rec := range added {
		p.log.Info("added", "record", rec)
	}

	err = p.writeRecs(ctx, recs)
	if err != nil {
		return err
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
			r, err := p.readSourceRecs(ctx, source)

			lock.Lock()
			if err != nil {
				errs = append(errs, err)
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

func (p *Prov) readSourceRecs(ctx context.Context, src source.Source) ([]dns.Record, error) {
	start := time.Now()
	defer func() {
		metricSourceFetchTime.WithLabelValues(src.ID()).Set(time.Since(start).Seconds())
	}()

	res, err := src.Read(ctx)
	if err != nil {
		err = fmt.Errorf("%s: %w", src.ID(), err)

		metricSourceRecords.WithLabelValues(src.ID()).Set(0)
		metricSourceStatus.WithLabelValues(src.ID()).Set(0)
	} else {
		metricSourceRecords.WithLabelValues(src.ID()).Set(float64(len(res)))
		metricSourceStatus.WithLabelValues(src.ID()).Set(1)
	}

	return res, err
}

func (p *Prov) applyModifier(ctx context.Context, mod modifier.Modifier, recs []dns.Record) ([]dns.Record, error) {
	start := time.Now()
	defer func() {
		metricModifierApplyTime.WithLabelValues(mod.ID()).Set(time.Since(start).Seconds())
	}()

	var err error
	recs, err = mod.Modify(ctx, recs)
	if err != nil {
		err = fmt.Errorf("%s: %w", mod.ID(), err)
		metricModifierStatus.WithLabelValues(mod.ID()).Set(0)
	} else {
		metricModifierStatus.WithLabelValues(mod.ID()).Set(1)
	}

	return recs, err
}

func (p *Prov) writeRecs(ctx context.Context, recs []dns.Record) error {
	var lock sync.Mutex
	var errs []error

	var wg sync.WaitGroup

	wg.Add(len(p.sinks))
	for _, sink := range p.sinks {
		go func() {
			err := p.writeSinkRecs(ctx, sink, recs)

			lock.Lock()
			if err != nil {
				errs = append(errs, err)
			}
			lock.Unlock()

			wg.Done()
		}()
	}
	wg.Wait()

	return errors.Join(errs...)
}

func (p *Prov) writeSinkRecs(ctx context.Context, snk sink.Sink, recs []dns.Record) error {
	start := time.Now()
	defer func() {
		metricSinkWriteTime.WithLabelValues(snk.ID()).Set(time.Since(start).Seconds())
	}()

	err := snk.Write(ctx, recs)
	if err != nil {
		err = fmt.Errorf("%s: %w", snk.ID(), err)
		metricSinkStatus.WithLabelValues(snk.ID()).Set(0)
	} else {
		metricSinkStatus.WithLabelValues(snk.ID()).Set(1)
	}

	return err
}
