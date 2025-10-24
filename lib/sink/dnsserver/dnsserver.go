package dnsserver

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/netip"
	"sync"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	dnssrv "github.com/miekg/dns"
)

type DNSServer struct {
	log *slog.Logger
	cfg Config

	lock  sync.RWMutex
	store *store
}

func New(log *slog.Logger, cfg Config) (*DNSServer, error) {
	d := &DNSServer{
		log:   log.With("sink", "dnsserver"),
		cfg:   cfg,
		store: &store{},
	}
	go d.start()

	return d, nil
}

func (d *DNSServer) start() {
	srv := dnssrv.Server{
		Net:     "udp",
		Addr:    d.cfg.ListenAddr,
		Handler: dnssrv.HandlerFunc(d.handler),
	}

	d.log.Info("listening", "addr", d.cfg.ListenAddr)

	err := srv.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

func (d *DNSServer) Write(ctx context.Context, records []dns.Record) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.store.reset()

	for _, rec := range records {
		ok, err := d.cfg.Filter.Match(rec)
		if err != nil {
			return err
		}

		if !ok {
			continue
		}

		d.store.add(rec)
	}

	return nil
}

func (d *DNSServer) handler(w dnssrv.ResponseWriter, req *dnssrv.Msg) {
	res := new(dnssrv.Msg)
	res.SetReply(req)
	res.Authoritative = true

	for _, q := range req.Question {
		d.answer(q, res)
	}

	err := w.WriteMsg(res)
	if err != nil {
		d.log.Warn(err.Error())
	}
}

func (d *DNSServer) answer(q dnssrv.Question, res *dnssrv.Msg) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	// CNAME handling
	cnames := d.store.get(q.Name, dns.CNAME)
	if (q.Qtype == dnssrv.TypeA || q.Qtype == dnssrv.TypeAAAA) && len(cnames) > 0 {
		target := cnames[0].Target

		res.Answer = append(res.Answer, &dnssrv.CNAME{
			Hdr: dnssrv.RR_Header{
				Name:   q.Name,
				Rrtype: dnssrv.TypeA,
				Class:  dnssrv.ClassINET,
				Ttl:    30,
			},
			Target: target,
		})

		d.answer(dnssrv.Question{
			Name:   target,
			Qtype:  q.Qtype,
			Qclass: q.Qclass,
		}, res)

		return
	}

	switch q.Qtype {
	case dnssrv.TypeA:
		for _, rec := range d.store.get(q.Name, dns.A) {
			res.Answer = append(res.Answer, &dnssrv.A{
				Hdr: dnssrv.RR_Header{
					Name:   q.Name,
					Rrtype: dnssrv.TypeA,
					Class:  dnssrv.ClassINET,
					Ttl:    30,
				},
				A: addrNetipToNetDotIP(rec.Address),
			})
		}

	case dnssrv.TypeAAAA:
		for _, rec := range d.store.get(q.Name, dns.AAAA) {
			res.Answer = append(res.Answer, &dnssrv.AAAA{
				Hdr: dnssrv.RR_Header{
					Name:   q.Name,
					Rrtype: dnssrv.TypeAAAA,
					Class:  dnssrv.ClassINET,
					Ttl:    30,
				},
				AAAA: addrNetipToNetDotIP(rec.Address),
			})
		}

	case dnssrv.TypePTR:
		for _, rec := range d.store.get(q.Name, dns.PTR) {
			res.Answer = append(res.Answer, &dnssrv.PTR{
				Hdr: dnssrv.RR_Header{
					Name:   q.Name,
					Rrtype: dnssrv.TypePTR,
					Class:  dnssrv.ClassINET,
					Ttl:    30,
				},
				Ptr: rec.Ptr,
			})
		}

	case dnssrv.TypeSRV:
		for _, rec := range d.store.get(q.Name, dns.SRV) {
			res.Answer = append(res.Answer, &dnssrv.SRV{
				Hdr: dnssrv.RR_Header{
					Name:   q.Name,
					Rrtype: dnssrv.TypePTR,
					Class:  dnssrv.ClassINET,
					Ttl:    30,
				},
				Priority: rec.Priority,
				Weight:   rec.Weight,
				Port:     rec.Port,
				Target:   rec.Target,
			})
		}

	case dnssrv.TypeMX:
		for _, rec := range d.store.get(q.Name, dns.MX) {
			res.Answer = append(res.Answer, &dnssrv.MX{
				Hdr: dnssrv.RR_Header{
					Name:   q.Name,
					Rrtype: dnssrv.TypePTR,
					Class:  dnssrv.ClassINET,
					Ttl:    30,
				},
				Preference: rec.Preference,
				Mx:         rec.Mx,
			})
		}
	}
}

func addrNetipToNetDotIP(addr netip.Addr) net.IP {
	s := addr.AsSlice()
	return net.IP(s)
}
