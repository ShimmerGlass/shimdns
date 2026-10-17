package prov

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
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

	sourceRecs map[source.Source][]dns.Record // last successful read of each source

	now func() time.Time // injectable for tests

	// tracked holds every record seen and not yet forgotten, including the
	// ones that disappeared from the sources less than the longest removal
	// grace ago.
	tracked map[string]*trackedRec
	// sinkPrev is what each sink was last given, to know when to write.
	sinkPrev map[sink.Sink][]dns.Record
}

type trackedRec struct {
	rec          dns.Record
	missingSince time.Time // zero while the sources still return the record
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
		sourceRecs: map[source.Source][]dns.Record{},
		now:        time.Now,
		tracked:    map[string]*trackedRec{},
		sinkPrev:   map[sink.Sink][]dns.Record{},
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

	p.track(recs)

	err = p.writeRecs(ctx, recs)
	if err != nil {
		return err
	}

	return nil
}

// track updates the records seen with the ones just read, and logs what
// was added or removed. A record that disappeared is only removed once it
// has been absent for the longest removal grace of the sinks; until then
// the sinks with a longer grace keep being given it (see sinkRecs). When
// a record with the same name and type is present, the missing one is a
// change of data, not a blip: it is removed at once.
func (p *Prov) track(recs []dns.Record) {
	now := p.now()

	present := map[string]bool{}
	live := map[string]bool{} // name and type with at least one record
	for _, rec := range recs {
		k := recKey(rec)
		present[k] = true
		live[nameType(rec)] = true

		t, ok := p.tracked[k]
		switch {
		case !ok:
			p.log.Info("record added", "record", rec)
			p.tracked[k] = &trackedRec{rec: rec}
		case !t.missingSince.IsZero():
			p.log.Debug("record is back", "record", rec, "missing", now.Sub(t.missingSince))
			t.missingSince = time.Time{}
		}
	}

	maxGrace := p.maxGrace()
	for k, t := range p.tracked {
		if present[k] {
			continue
		}

		if t.missingSince.IsZero() {
			t.missingSince = now
			if maxGrace > 0 && !live[nameType(t.rec)] {
				p.log.Debug("record is missing, keeping it", "record", t.rec, "grace", maxGrace)
			}
		}

		if live[nameType(t.rec)] || now.Sub(t.missingSince) >= maxGrace {
			p.log.Info("record removed", "record", t.rec)
			delete(p.tracked, k)
		}
	}
}

// sinkRecs returns the records to give to a sink: the current ones, plus
// the missing ones that are still within the sink's removal grace.
func (p *Prov) sinkRecs(snk sink.Sink, recs []dns.Record) []dns.Record {
	grace := sinkGrace(snk)
	if grace <= 0 {
		return recs
	}

	now := p.now()
	var kept []string
	for k, t := range p.tracked {
		if !t.missingSince.IsZero() && now.Sub(t.missingSince) < grace {
			kept = append(kept, k)
		}
	}
	if len(kept) == 0 {
		return recs
	}

	slices.Sort(kept) // stable output
	out := append([]dns.Record{}, recs...)
	for _, k := range kept {
		out = append(out, p.tracked[k].rec)
	}
	return out
}

func (p *Prov) maxGrace() time.Duration {
	var max time.Duration
	for _, snk := range p.sinks {
		max = slices.Max([]time.Duration{max, sinkGrace(snk)})
	}
	return max
}

func sinkGrace(snk sink.Sink) time.Duration {
	if g, ok := snk.(sink.RemovalGracer); ok {
		return g.RemovalGrace()
	}
	return 0
}

func recKey(rec dns.Record) string {
	return fmt.Sprintf("%+v", rec)
}

func nameType(rec dns.Record) string {
	return rec.Type + " " + rec.Name
}

// readRecs reads records from all sources. When a source fails, the records
// from its last successful read are used instead. An error is returned only
// if a source failed and never had a successful read, in which case the
// update must be skipped to avoid removing its records from the sinks.
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
				last, ok := p.sourceRecs[source]
				if ok {
					p.log.Error("source read failed, using last known records", "err", err, "records", len(last))
					recs = append(recs, last...)
				} else {
					errs = append(errs, err)
				}
			} else {
				p.sourceRecs[source] = r
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

	lock.Lock()
	for _, sink := range p.sinks {
		sinkRecs := p.sinkRecs(sink, recs)

		added, removed := diff(p.sinkPrev[sink], sinkRecs)
		if len(added) == 0 && len(removed) == 0 && p.sinkStatus[sink] {
			continue
		}
		p.sinkPrev[sink] = sinkRecs

		wg.Add(1)
		go func() {
			p.log.Info("updating sink", "sink", sink.ID())
			err := p.writeSinkRecs(ctx, sink, sinkRecs)

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
