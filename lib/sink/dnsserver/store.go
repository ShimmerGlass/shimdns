package dnsserver

import (
	"strings"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

// store indexes records by name and type. Names are case-insensitive.
type store struct {
	recs map[string]map[string][]dns.Record

	// names holds every name that has records, and all their parent names
	names map[string]struct{}
}

func (s *store) reset() {
	s.recs = map[string]map[string][]dns.Record{}
	s.names = map[string]struct{}{}
}

func (s *store) add(rec dns.Record) {
	name := strings.ToLower(rec.Name)

	for n := name; n != ""; {
		s.names[n] = struct{}{}

		_, parent, ok := strings.Cut(n, ".")
		if !ok {
			break
		}
		n = parent
	}

	nameRecs, ok := s.recs[name]
	if !ok {
		nameRecs = map[string][]dns.Record{}
		s.recs[name] = nameRecs
	}

	nameRecs[rec.Type] = append(nameRecs[rec.Type], rec)
}

func (s *store) get(name string, t string) []dns.Record {
	recs, ok := s.recs[strings.ToLower(name)]
	if !ok {
		return nil
	}

	return recs[t]
}

// has reports whether name exists, either because it has records or because
// names below it do (empty non-terminal).
func (s *store) has(name string) bool {
	_, ok := s.names[strings.ToLower(name)]
	return ok
}
