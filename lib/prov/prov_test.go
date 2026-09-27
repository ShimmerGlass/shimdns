package prov

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"sync"
	"testing"

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
