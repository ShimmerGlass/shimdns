package prov

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/ShimmerGlass/shimdns/lib/modifier"
	"github.com/ShimmerGlass/shimdns/lib/sink"
	"github.com/ShimmerGlass/shimdns/lib/source"
)

type Prov struct {
	log *slog.Logger

	interval time.Duration

	sources    []source.Source
	modifiers  []modifier.Modifier
	sinks      []sink.Sink
	sinkStatus map[sink.Sink]bool // was the last write successful

	prev []dns.Record
}

func New(log *slog.Logger, interval time.Duration, sources []source.Source, modifiers []modifier.Modifier, sinks []sink.Sink) (*Prov, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("invalid interval %s", interval)
	}

	return &Prov{
		log:        log,
		interval:   interval,
		sources:    sources,
		modifiers:  modifiers,
		sinks:      sinks,
		sinkStatus: map[sink.Sink]bool{},
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

	added, removed := diff(p.prev, recs)
	changed := len(removed) > 0 || len(added) > 0

	for _, rec := range removed {
		p.log.Info("record removed", "record", rec)
	}

	for _, rec := range added {
		p.log.Info("record added", "record", rec)
	}

	p.prev = recs

	err = p.writeRecs(ctx, recs, changed)
	if err != nil {
		return err
	}

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

func (p *Prov) writeRecs(ctx context.Context, recs []dns.Record, changed bool) error {
	var lock sync.Mutex
	var errs []error

	var wg sync.WaitGroup

	lock.Lock()
	for _, sink := range p.sinks {
		if !changed && p.sinkStatus[sink] {
			continue
		}

		wg.Add(1)
		go func() {
			p.log.Info("updating sink", "sink", sink.ID())
			err := p.writeSinkRecs(ctx, sink, recs)

			lock.Lock()
			if err != nil {
				errs = append(errs, err)
			}
			p.sinkStatus[sink] = err == nil
			lock.Unlock()

			wg.Done()
		}()
	}
	lock.Unlock()
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

func diff(old, new []dns.Record) (added, removed []dns.Record) {
	for _, o := range old {
		found := false
		for _, n := range new {
			found = reflect.DeepEqual(o, n)
			if found {
				break
			}
		}

		if !found {
			removed = append(removed, o)
		}
	}

	for _, n := range new {
		found := false
		for _, o := range old {
			found = reflect.DeepEqual(o, n)
			if found {
				break
			}
		}

		if !found {
			added = append(added, n)
		}
	}

	return
}
