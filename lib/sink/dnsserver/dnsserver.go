package dnsserver

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	dnssrv "github.com/miekg/dns"
)

const Type = "dnsserver"

// maxCNAMEChain bounds the CNAME chains followed in the store
const maxCNAMEChain = 8

// upstreamTimeout bounds the query for a CNAME target outside the store
const upstreamTimeout = 2 * time.Second

type DNSServer struct {
	log *slog.Logger
	cfg Config
	id  string

	lock  sync.RWMutex
	store *store
}

func New(log *slog.Logger, cfg Config, id string) (*DNSServer, error) {
	cfg.Upstream = withDefaultPort(cfg.Upstream)

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

		// ALIAS is flattened by the provider that hosts the zone, it can't be served here
		if rec.Type == dns.ALIAS {
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

	// upstream answers can be large, keep within what the client accepts over UDP
	size := dnssrv.MinMsgSize
	if opt := req.IsEdns0(); opt != nil {
		size = max(size, int(opt.UDPSize()))
	}
	res.Truncate(size)

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

	if external := d.answerLocal(q, res); external != "" {
		d.answerUpstream(external, q.Qtype, res)
	}
}

// answerLocal answers q from the store. When a CNAME chain leads out of the
// store it returns the external target, whose records are not in res yet.
func (d *DNSServer) answerLocal(q dnssrv.Question, res *dnssrv.Msg) string {
	d.lock.RLock()
	defer d.lock.RUnlock()

	name := q.Name

	if !d.store.has(name) {
		res.Rcode = dnssrv.RcodeNameError
		return ""
	}

	// follow CNAME chains within the store, the other records are then
	// looked up on the final target
	// a loop runs past the limit too
	answerStart := len(res.Answer)
	hops := 0
	for {
		cname, ok := d.resolveCNAME(name)
		if !ok {
			break
		}

		hops++
		if hops > maxCNAMEChain {
			d.log.Warn("cname chain too long or looping", "name", q.Name)
			res.Answer = res.Answer[:answerStart]
			res.Rcode = dnssrv.RcodeServerFailure
			return ""
		}

		res.Answer = append(res.Answer, cname)

		if q.Qtype == dnssrv.TypeCNAME {
			return ""
		}

		name = cname.Target
	}

	if hops > 0 && !d.store.hasRecords(name) {
		return name
	}

	switch q.Qtype {
	case dnssrv.TypeA:
		res.Answer = appendSeq(res.Answer, d.resolveA(name))

	case dnssrv.TypeAAAA:
		res.Answer = appendSeq(res.Answer, d.resolveAAAA(name))

	case dnssrv.TypePTR:
		res.Answer = appendSeq(res.Answer, d.resolvePTR(name))

	case dnssrv.TypeSRV:
		res.Answer = appendSeq(res.Answer, d.resolveSRV(name))

	case dnssrv.TypeMX:
		res.Answer = appendSeq(res.Answer, d.resolveMX(name))

	case dnssrv.TypeTXT:
		res.Answer = appendSeq(res.Answer, d.resolveTXT(name))

	case dnssrv.TypeCAA:
		res.Answer = appendSeq(res.Answer, d.resolveCAA(name))

	case dnssrv.TypeHTTPS:
		https, ok := d.resolveHTTPS(name)
		if !ok {
			return ""
		}

		res.Answer = append(res.Answer, https)

		target := https.Target
		if target == "." {
			target = name
		}

		res.Extra = appendSeq(res.Extra, d.resolveA(target))
		res.Extra = appendSeq(res.Extra, d.resolveAAAA(target))
	}

	return ""
}

// answerUpstream completes a CNAME chain that leads out of the store with the
// upstream's answer for its target, as a recursive resolver would. Stub
// resolvers that don't chase CNAMEs themselves (Android) need it. A failed
// upstream answers SERVFAIL so clients retry instead of caching the chain alone.
func (d *DNSServer) answerUpstream(name string, qtype uint16, res *dnssrv.Msg) {
	if d.cfg.Upstream == "" || qtype == dnssrv.TypeANY {
		return
	}

	req := new(dnssrv.Msg)
	req.SetQuestion(name, qtype)
	req.SetEdns0(dnssrv.DefaultMsgSize, false)

	client := &dnssrv.Client{Timeout: upstreamTimeout}
	up, _, err := client.Exchange(req, d.cfg.Upstream)
	if err == nil && up.Truncated {
		client.Net = "tcp"
		up, _, err = client.Exchange(req, d.cfg.Upstream)
	}
	if err != nil {
		d.log.Warn("upstream query failed", "name", name, "upstream", d.cfg.Upstream, "err", err)
		res.Rcode = dnssrv.RcodeServerFailure
		return
	}

	switch up.Rcode {
	case dnssrv.RcodeSuccess, dnssrv.RcodeNameError:
		res.Answer = append(res.Answer, up.Answer...)
		res.Rcode = up.Rcode
	default:
		res.Rcode = dnssrv.RcodeServerFailure
	}
}

// withDefaultPort adds the DNS port to an address given without one.
func withDefaultPort(addr string) string {
	if addr == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	return net.JoinHostPort(strings.Trim(addr, "[]"), "53")
}

func addrNetipToNetDotIP(addr netip.Addr) net.IP {
	s := addr.AsSlice()
	return net.IP(s)
}
