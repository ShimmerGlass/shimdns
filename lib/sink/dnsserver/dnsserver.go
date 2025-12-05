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
	cname, cnameOk := d.resolveCNAME(q.Name)
	if (q.Qtype == dnssrv.TypeA || q.Qtype == dnssrv.TypeAAAA) && cnameOk {
		target := cname.Target

		res.Answer = append(res.Answer, &dnssrv.CNAME{
			Hdr: dnssrv.RR_Header{
				Name:   q.Name,
				Rrtype: dnssrv.TypeCNAME,
				Class:  dnssrv.ClassINET,
				Ttl:    30,
			},
			Target: target,
		})

		res.Extra = appendSeq(res.Extra, d.resolveA(target))
		res.Extra = appendSeq(res.Extra, d.resolveAAAA(target))

		return
	}

	switch q.Qtype {
	case dnssrv.TypeA:
		res.Answer = appendSeq(res.Answer, d.resolveA(q.Name))

	case dnssrv.TypeAAAA:
		res.Answer = appendSeq(res.Answer, d.resolveAAAA(q.Name))

	case dnssrv.TypePTR:
		res.Answer = appendSeq(res.Answer, d.resolvePTR(q.Name))

	case dnssrv.TypeSRV:
		res.Answer = appendSeq(res.Answer, d.resolveSRV(q.Name))

	case dnssrv.TypeMX:
		res.Answer = appendSeq(res.Answer, d.resolveMX(q.Name))

	case dnssrv.TypeHTTPS:
		https, ok := d.resolveHTTPS(q.Name)
		if !ok {
			return
		}

		res.Answer = append(res.Answer, https)

		target := https.Target
		if target == "." {
			target = q.Name
		}

		res.Extra = appendSeq(res.Extra, d.resolveA(target))
		res.Extra = appendSeq(res.Extra, d.resolveAAAA(target))
	}
}

func addrNetipToNetDotIP(addr netip.Addr) net.IP {
	s := addr.AsSlice()
	return net.IP(s)
}
