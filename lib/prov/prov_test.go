package prov

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/ShimmerGlass/shimdns/lib/modifier"
	"github.com/ShimmerGlass/shimdns/lib/sink"
	"github.com/ShimmerGlass/shimdns/lib/source"
	"github.com/stretchr/testify/require"
)

type fakeSource struct {
	id   string
	recs []dns.Record
	err  error
}

func (s *fakeSource) ID() string { return s.id }

func (s *fakeSource) Read(ctx context.Context) ([]dns.Record, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]dns.Record{}, s.recs...), nil
}

type fakeSink struct {
	id     string
	err    error
	lock   sync.Mutex
	writes [][]dns.Record
}

func (s *fakeSink) ID() string { return s.id }

func (s *fakeSink) Write(ctx context.Context, recs []dns.Record) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.writes = append(s.writes, append([]dns.Record{}, recs...))
	return s.err
}

func (s *fakeSink) last() []dns.Record {
	if len(s.writes) == 0 {
		return nil
	}
	return s.writes[len(s.writes)-1]
}

type fakeModifier struct {
	id string
	fn func([]dns.Record) ([]dns.Record, error)
}

func (m *fakeModifier) ID() string { return m.id }

func (m *fakeModifier) Modify(ctx context.Context, recs []dns.Record) ([]dns.Record, error) {
	return m.fn(recs)
}

func rec(name, addr string) dns.Record {
	return dns.Record{Type: dns.A, Name: name, TTL: 60, Address: netip.MustParseAddr(addr)}
}

func newTestProv(t *testing.T, sources []source.Source, modifiers []modifier.Modifier, sinks []sink.Sink) *Prov {
	t.Helper()

	p, err := New(slog.Default(), 1, sources, modifiers, sinks)
	require.NoError(t, err)

	return p
}

func TestNewInvalidInterval(t *testing.T) {
	_, err := New(slog.Default(), 0, nil, nil, nil)
	require.Error(t, err)

	_, err = New(slog.Default(), -1, nil, nil, nil)
	require.Error(t, err)
}

func TestRunOnce(t *testing.T) {
	src1 := &fakeSource{id: "src1", recs: []dns.Record{rec("a.", "10.0.0.1")}}
	src2 := &fakeSource{id: "src2", recs: []dns.Record{rec("b.", "10.0.0.2"), rec("c.", "10.0.0.3")}}
	snk1 := &fakeSink{id: "snk1"}
	snk2 := &fakeSink{id: "snk2"}

	p := newTestProv(t, []source.Source{src1, src2}, nil, []sink.Sink{snk1, snk2})

	require.NoError(t, p.runOnce(context.Background()))

	want := []dns.Record{rec("a.", "10.0.0.1"), rec("b.", "10.0.0.2"), rec("c.", "10.0.0.3")}
	require.ElementsMatch(t, want, snk1.last())
	require.ElementsMatch(t, want, snk2.last())
}

func TestModifiersAppliedInOrder(t *testing.T) {
	src := &fakeSource{id: "src", recs: []dns.Record{rec("a.", "10.0.0.1")}}
	snk := &fakeSink{id: "snk"}

	appendName := func(suffix string) *fakeModifier {
		return &fakeModifier{id: suffix, fn: func(recs []dns.Record) ([]dns.Record, error) {
			for i := range recs {
				recs[i].Name += suffix
			}
			return recs, nil
		}}
	}

	p := newTestProv(t, []source.Source{src}, []modifier.Modifier{appendName("x"), appendName("y")}, []sink.Sink{snk})

	require.NoError(t, p.runOnce(context.Background()))
	require.Equal(t, []dns.Record{rec("a.xy", "10.0.0.1")}, snk.last())
}

func TestModifierError(t *testing.T) {
	src := &fakeSource{id: "src", recs: []dns.Record{rec("a.", "10.0.0.1")}}
	snk := &fakeSink{id: "snk"}
	mod := &fakeModifier{id: "mod", fn: func([]dns.Record) ([]dns.Record, error) {
		return nil, errors.New("boom")
	}}

	p := newTestProv(t, []source.Source{src}, []modifier.Modifier{mod}, []sink.Sink{snk})

	err := p.runOnce(context.Background())
	require.ErrorContains(t, err, "mod")
	require.ErrorContains(t, err, "boom")
	require.Empty(t, snk.writes)
}

func TestUnchangedSkipsHealthySinks(t *testing.T) {
	src := &fakeSource{id: "src", recs: []dns.Record{rec("a.", "10.0.0.1")}}
	snk := &fakeSink{id: "snk"}

	p := newTestProv(t, []source.Source{src}, nil, []sink.Sink{snk})

	require.NoError(t, p.runOnce(context.Background()))
	require.NoError(t, p.runOnce(context.Background()))
	require.Len(t, snk.writes, 1)

	src.recs = append(src.recs, rec("b.", "10.0.0.2"))
	require.NoError(t, p.runOnce(context.Background()))
	require.Len(t, snk.writes, 2)
	require.ElementsMatch(t, src.recs, snk.last())
}

func TestFailedSinkRetried(t *testing.T) {
	src := &fakeSource{id: "src", recs: []dns.Record{rec("a.", "10.0.0.1")}}
	good := &fakeSink{id: "good"}
	bad := &fakeSink{id: "bad", err: errors.New("down")}

	p := newTestProv(t, []source.Source{src}, nil, []sink.Sink{good, bad})

	err := p.runOnce(context.Background())
	require.ErrorContains(t, err, "bad")
	require.Len(t, good.writes, 1)
	require.Len(t, bad.writes, 1)

	// nothing changed: the healthy sink is skipped, the failed one is retried
	err = p.runOnce(context.Background())
	require.Error(t, err)
	require.Len(t, good.writes, 1)
	require.Len(t, bad.writes, 2)

	bad.err = nil
	require.NoError(t, p.runOnce(context.Background()))
	require.Len(t, bad.writes, 3)

	require.NoError(t, p.runOnce(context.Background()))
	require.Len(t, bad.writes, 3)
}

func TestSourceFailureKeepsLastRecords(t *testing.T) {
	src1 := &fakeSource{id: "src1", recs: []dns.Record{rec("a.", "10.0.0.1")}}
	src2 := &fakeSource{id: "src2", recs: []dns.Record{rec("b.", "10.0.0.2")}}
	snk := &fakeSink{id: "snk"}

	p := newTestProv(t, []source.Source{src1, src2}, nil, []sink.Sink{snk})

	require.NoError(t, p.runOnce(context.Background()))
	require.ElementsMatch(t, []dns.Record{rec("a.", "10.0.0.1"), rec("b.", "10.0.0.2")}, snk.last())

	// src1 fails while src2 changes: src2 changes are applied, src1 records are kept
	src1.err = errors.New("down")
	src2.recs = []dns.Record{rec("b.", "10.0.0.3")}

	require.NoError(t, p.runOnce(context.Background()))
	require.ElementsMatch(t, []dns.Record{rec("a.", "10.0.0.1"), rec("b.", "10.0.0.3")}, snk.last())

	// src1 recovers with new records
	src1.err = nil
	src1.recs = []dns.Record{rec("c.", "10.0.0.4")}

	require.NoError(t, p.runOnce(context.Background()))
	require.ElementsMatch(t, []dns.Record{rec("c.", "10.0.0.4"), rec("b.", "10.0.0.3")}, snk.last())
}

func TestSourceFailureEmptyLastRecords(t *testing.T) {
	src1 := &fakeSource{id: "src1"}
	src2 := &fakeSource{id: "src2", recs: []dns.Record{rec("b.", "10.0.0.2")}}
	snk := &fakeSink{id: "snk"}

	p := newTestProv(t, []source.Source{src1, src2}, nil, []sink.Sink{snk})
	require.NoError(t, p.runOnce(context.Background()))

	// a source whose last successful read was empty is still considered loaded
	src1.err = errors.New("down")
	src2.recs = []dns.Record{rec("b.", "10.0.0.3")}

	require.NoError(t, p.runOnce(context.Background()))
	require.Equal(t, []dns.Record{rec("b.", "10.0.0.3")}, snk.last())
}

func TestSourceNeverLoaded(t *testing.T) {
	src1 := &fakeSource{id: "src1", err: errors.New("down")}
	src2 := &fakeSource{id: "src2", recs: []dns.Record{rec("b.", "10.0.0.2")}}
	snk := &fakeSink{id: "snk"}

	p := newTestProv(t, []source.Source{src1, src2}, nil, []sink.Sink{snk})

	err := p.runOnce(context.Background())
	require.ErrorContains(t, err, "src1")
	require.ErrorContains(t, err, "down")
	require.Empty(t, snk.writes)

	src1.err = nil
	src1.recs = []dns.Record{rec("a.", "10.0.0.1")}

	require.NoError(t, p.runOnce(context.Background()))
	require.ElementsMatch(t, []dns.Record{rec("a.", "10.0.0.1"), rec("b.", "10.0.0.2")}, snk.last())
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func graceProv(t *testing.T, grace time.Duration, sources []source.Source, snk sink.Sink) (*Prov, *clock) {
	t.Helper()

	p := newTestProv(t, sources, nil, []sink.Sink{sink.WithRemovalGrace(snk, grace)})
	c := &clock{t: time.Unix(1000, 0)}
	p.now = c.now

	return p, c
}

func TestGraceRecordBack(t *testing.T) {
	src := &fakeSource{id: "src", recs: []dns.Record{rec("a.", "10.0.0.1"), rec("b.", "10.0.0.2")}}
	snk := &fakeSink{id: "snk"}
	p, c := graceProv(t, time.Minute, []source.Source{src}, snk)
	ctx := context.Background()

	require.NoError(t, p.runOnce(ctx))
	require.Len(t, snk.writes, 1)

	// a. disappears: still published, and the sink is not written
	src.recs = []dns.Record{rec("b.", "10.0.0.2")}
	c.advance(10 * time.Second)
	require.NoError(t, p.runOnce(ctx))
	require.Len(t, snk.writes, 1)

	// and comes back within the grace period
	c.advance(10 * time.Second)
	src.recs = []dns.Record{rec("a.", "10.0.0.1"), rec("b.", "10.0.0.2")}
	require.NoError(t, p.runOnce(ctx))

	// the grace period restarts from the next disappearance, not the first
	src.recs = []dns.Record{rec("b.", "10.0.0.2")}
	c.advance(50 * time.Second)
	require.NoError(t, p.runOnce(ctx))
	c.advance(50 * time.Second)
	require.NoError(t, p.runOnce(ctx))

	require.Len(t, snk.writes, 1)
}

func TestGraceExpires(t *testing.T) {
	src := &fakeSource{id: "src", recs: []dns.Record{rec("a.", "10.0.0.1"), rec("b.", "10.0.0.2")}}
	snk := &fakeSink{id: "snk"}
	p, c := graceProv(t, time.Minute, []source.Source{src}, snk)
	ctx := context.Background()

	require.NoError(t, p.runOnce(ctx))

	src.recs = []dns.Record{rec("b.", "10.0.0.2")}
	c.advance(5 * time.Second)
	require.NoError(t, p.runOnce(ctx))
	c.advance(59 * time.Second)
	require.NoError(t, p.runOnce(ctx))
	require.Len(t, snk.writes, 1)

	c.advance(time.Second)
	require.NoError(t, p.runOnce(ctx))
	require.Len(t, snk.writes, 2)
	require.Equal(t, []dns.Record{rec("b.", "10.0.0.2")}, snk.last())
	require.Len(t, p.tracked, 1)

	// gone for good: no further write
	c.advance(time.Hour)
	require.NoError(t, p.runOnce(ctx))
	require.Len(t, snk.writes, 2)
}

func TestGraceAddressChange(t *testing.T) {
	src := &fakeSource{id: "src", recs: []dns.Record{rec("a.", "10.0.0.1")}}
	snk := &fakeSink{id: "snk"}
	p, c := graceProv(t, time.Hour, []source.Source{src}, snk)
	ctx := context.Background()

	require.NoError(t, p.runOnce(ctx))

	// the new address replaces the old one at once, without an empty zone:
	// a single write holds the new record
	src.recs = []dns.Record{rec("a.", "10.0.0.9")}
	c.advance(time.Second)
	require.NoError(t, p.runOnce(ctx))
	require.Len(t, snk.writes, 2)
	require.Equal(t, []dns.Record{rec("a.", "10.0.0.9")}, snk.last())

	// the old one does not come back when the new one goes away
	src.recs = nil
	c.advance(time.Second)
	require.NoError(t, p.runOnce(ctx))
	require.Equal(t, []dns.Record{rec("a.", "10.0.0.9")}, snk.last())
}

func TestGraceFailingSource(t *testing.T) {
	src := &fakeSource{id: "src", recs: []dns.Record{rec("a.", "10.0.0.1")}}
	snk := &fakeSink{id: "snk"}
	p, c := graceProv(t, time.Minute, []source.Source{src}, snk)
	ctx := context.Background()

	require.NoError(t, p.runOnce(ctx))

	// while the source fails its last records are current, whatever the time
	src.err = errors.New("down")
	c.advance(time.Hour)
	require.NoError(t, p.runOnce(ctx))
	c.advance(time.Hour)
	require.NoError(t, p.runOnce(ctx))
	require.Len(t, snk.writes, 1)

	// it recovers without the record: the grace period starts now
	src.err = nil
	src.recs = nil
	require.NoError(t, p.runOnce(ctx))
	c.advance(30 * time.Second)
	require.NoError(t, p.runOnce(ctx))
	require.Len(t, snk.writes, 1)

	c.advance(30 * time.Second)
	require.NoError(t, p.runOnce(ctx))
	require.Len(t, snk.writes, 2)
	require.Empty(t, snk.last())
}

func TestGracePerSink(t *testing.T) {
	src := &fakeSource{id: "src", recs: []dns.Record{rec("a.", "10.0.0.1")}}
	local := &fakeSink{id: "local"}
	public := &fakeSink{id: "public"}

	p := newTestProv(t, []source.Source{src}, nil,
		[]sink.Sink{local, sink.WithRemovalGrace(public, time.Minute)})
	c := &clock{t: time.Unix(1000, 0)}
	p.now = c.now
	ctx := context.Background()

	require.NoError(t, p.runOnce(ctx))

	src.recs = nil
	c.advance(10 * time.Second)
	require.NoError(t, p.runOnce(ctx))
	require.Empty(t, local.last())
	require.Len(t, local.writes, 2)
	require.Len(t, public.writes, 1)

	c.advance(time.Minute)
	require.NoError(t, p.runOnce(ctx))
	require.Len(t, public.writes, 2)
	require.Empty(t, public.last())
}
