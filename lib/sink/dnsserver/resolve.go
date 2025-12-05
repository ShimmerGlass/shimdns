package dnsserver

import (
	"iter"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	dnssrv "github.com/miekg/dns"
)

func (d *DNSServer) resolve(name, typ string, mapFn func(dns.Record) dnssrv.RR) iter.Seq[dnssrv.RR] {
	return func(yield func(dnssrv.RR) bool) {
		for _, rec := range d.store.get(name, typ) {
			if !yield(mapFn(rec)) {
				return
			}
		}
	}
}

func (d *DNSServer) resolveA(name string) iter.Seq[dnssrv.RR] {
	return d.resolve(name, dns.A, func(rec dns.Record) dnssrv.RR {
		return &dnssrv.A{
			Hdr: dnssrv.RR_Header{
				Name:   name,
				Rrtype: dnssrv.TypeA,
				Class:  dnssrv.ClassINET,
				Ttl:    uint32(rec.TTL),
			},
			A: addrNetipToNetDotIP(rec.Address),
		}
	})
}

func (d *DNSServer) resolveAAAA(name string) iter.Seq[dnssrv.RR] {
	return d.resolve(name, dns.AAAA, func(rec dns.Record) dnssrv.RR {
		return &dnssrv.AAAA{
			Hdr: dnssrv.RR_Header{
				Name:   name,
				Rrtype: dnssrv.TypeAAAA,
				Class:  dnssrv.ClassINET,
				Ttl:    uint32(rec.TTL),
			},
			AAAA: addrNetipToNetDotIP(rec.Address),
		}
	})
}

func (d *DNSServer) resolveCNAME(name string) (*dnssrv.CNAME, bool) {
	recs := d.store.get(name, dns.CNAME)
	if len(recs) == 0 {
		return nil, false
	}

	rec := recs[0]

	return &dnssrv.CNAME{
		Hdr: dnssrv.RR_Header{
			Name:   name,
			Rrtype: dnssrv.TypeCNAME,
			Class:  dnssrv.ClassINET,
			Ttl:    uint32(rec.TTL),
		},
		Target: rec.Target,
	}, true
}

func (d *DNSServer) resolvePTR(name string) iter.Seq[dnssrv.RR] {
	return d.resolve(name, dns.PTR, func(rec dns.Record) dnssrv.RR {
		return &dnssrv.PTR{
			Hdr: dnssrv.RR_Header{
				Name:   name,
				Rrtype: dnssrv.TypePTR,
				Class:  dnssrv.ClassINET,
				Ttl:    uint32(rec.TTL),
			},
			Ptr: rec.Ptr,
		}
	})
}

func (d *DNSServer) resolveSRV(name string) iter.Seq[dnssrv.RR] {
	return d.resolve(name, dns.SRV, func(rec dns.Record) dnssrv.RR {
		return &dnssrv.SRV{
			Hdr: dnssrv.RR_Header{
				Name:   name,
				Rrtype: dnssrv.TypeSRV,
				Class:  dnssrv.ClassINET,
				Ttl:    uint32(rec.TTL),
			},
			Priority: uint16(rec.Priority),
			Weight:   uint16(rec.Weight),
			Port:     uint16(rec.Port),
			Target:   rec.Target,
		}
	})
}

func (d *DNSServer) resolveMX(name string) iter.Seq[dnssrv.RR] {
	return d.resolve(name, dns.MX, func(rec dns.Record) dnssrv.RR {
		return &dnssrv.MX{
			Hdr: dnssrv.RR_Header{
				Name:   name,
				Rrtype: dnssrv.TypeMX,
				Class:  dnssrv.ClassINET,
				Ttl:    uint32(rec.TTL),
			},
			Preference: uint16(rec.Preference),
			Mx:         rec.Mx,
		}
	})
}

func (d *DNSServer) resolveHTTPS(name string) (*dnssrv.HTTPS, bool) {
	recs := d.store.get(name, dns.HTTPS)
	if len(recs) == 0 {
		return nil, false
	}

	rec := recs[0]

	drec := &dnssrv.HTTPS{
		SVCB: dnssrv.SVCB{
			Hdr: dnssrv.RR_Header{
				Name:   name,
				Rrtype: dnssrv.TypeHTTPS,
				Class:  dnssrv.ClassINET,
				Ttl:    uint32(rec.TTL),
			},
			Priority: uint16(rec.Priority),
			Target:   rec.Target,
		},
	}

	if len(rec.Alpn) > 0 {
		drec.Value = append(drec.Value, &dnssrv.SVCBAlpn{Alpn: rec.Alpn})
	}

	return drec, true
}

func appendSeq[T any](s []T, it iter.Seq[T]) []T {
	for v := range it {
		s = append(s, v)
	}
	return s
}
