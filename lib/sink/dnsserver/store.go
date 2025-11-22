package dnsserver

import (
	"github.com/ShimmerGlass/shimdns/lib/dns"
)

type store struct {
	recs map[string]map[string][]dns.Record
}

func (s *store) reset() {
	s.recs = map[string]map[string][]dns.Record{}
}

func (s *store) add(rec dns.Record) {
	nameRecs, ok := s.recs[rec.Name]
	if !ok {
		nameRecs = map[string][]dns.Record{}
		s.recs[rec.Name] = nameRecs
	}

	nameRecs[rec.Type] = append(nameRecs[rec.Type], rec)
}

func (s *store) get(name string, t string) []dns.Record {
	recs, ok := s.recs[name]
	if !ok {
		return nil
	}

	return recs[t]
}
