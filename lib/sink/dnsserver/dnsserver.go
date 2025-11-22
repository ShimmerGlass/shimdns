package dnsserver

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	dnssrv "github.com/miekg/dns"
	"github.com/samber/lo"
)

const Type = "dnsserver"

type DNSServer struct {
	log *slog.Logger
	cfg Config
	id  string

	lock  sync.RWMutex
	store *store
}

func New(log *slog.Logger, cfg Config, id string) (*DNSServer, error) {
	d := &DNSServer{
		log:   log,
		cfg:   cfg,
		id:    id,
		store: &store{},
	}
	go d.start()

	return d, nil
}

func (d *DNSServer) ID() string {
	return d.id
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
	start := time.Now()
	defer func() {
		metricResponseTime.WithLabelValues(d.id, dnssrv.TypeToString[q.Qtype]).Observe(time.Since(start).Seconds())
	}()

	d.lock.RLock()
	defer d.lock.RUnlock()

	// CNAME handling
	cnames := d.store.get(q.Name, dns.CNAME)
	if (q.Qtype == dnssrv.TypeA || q.Qtype == dnssrv.TypeAAAA) && len(cnames) > 0 {
		target := cnames[0].Target

		res.Answer = append(res.Answer, &dnssrv.CNAME{
			Hdr: dnssrv.RR_Header{
				Name:   q.Name,
				Rrtype: dnssrv.TypeCNAME,
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
					Rrtype: dnssrv.TypeSRV,
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

	case dnssrv.TypeHTTPS:
		for _, rec := range d.store.get(q.Name, dns.HTTPS) {
			drec := &dnssrv.HTTPS{
				SVCB: dnssrv.SVCB{
					Hdr: dnssrv.RR_Header{
						Name:   q.Name,
						Rrtype: dnssrv.TypeHTTPS,
						Class:  dnssrv.ClassINET,
						Ttl:    30,
					},
					Priority: rec.Priority,
					Target:   rec.Target,
				},
			}

			if len(rec.Alpn) > 0 {
				drec.Value = append(drec.Value, &dnssrv.SVCBAlpn{
					Alpn: rec.Alpn,
				})
			}

			if len(rec.IPv4Hint) > 0 {
				drec.Value = append(drec.Value, &dnssrv.SVCBIPv4Hint{
					Hint: lo.Map(rec.IPv4Hint, func(a netip.Addr, _ int) net.IP {
						return addrNetipToNetDotIP(a)
					}),
				})
			}

			if len(rec.IPv6Hint) > 0 {
				drec.Value = append(drec.Value, &dnssrv.SVCBIPv6Hint{
					Hint: lo.Map(rec.IPv6Hint, func(a netip.Addr, _ int) net.IP {
						return addrNetipToNetDotIP(a)
					}),
				})
			}

			res.Answer = append(res.Answer, drec)
		}
	}
}

func addrNetipToNetDotIP(addr netip.Addr) net.IP {
	s := addr.AsSlice()
	return net.IP(s)
}
