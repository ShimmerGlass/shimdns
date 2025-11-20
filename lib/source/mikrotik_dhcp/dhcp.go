package mikrotikdhcp

import (
	"context"
	"log/slog"
	"net/netip"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/ShimmerGlass/shimdns/lib/rest"
)

const Type = "mikrotik_dhcp"

type DHCP struct {
	log *slog.Logger
	cfg Config
	id  string
}

func New(log *slog.Logger, cfg Config, id string) (*DHCP, error) {
	if cfg.Name == "" {
		cfg.Name = cfg.URL
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	return &DHCP{
		log: log,
		cfg: cfg,
		id:  id,
	}, nil
}

func (d *DHCP) ID() string {
	return d.id
}

func (d *DHCP) Read(ctx context.Context) ([]dns.Record, error) {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.Timeout)
	defer cancel()

	leases, err := d.leases(ctx)
	if err != nil {
		return nil, err
	}

	recs := []dns.Record{}
	for _, lease := range leases {
		if lease.Comment == "" {
			continue
		}

		addrStr := lease.ActiveAddress
		if addrStr == "" {
			addrStr = lease.Address
		}

		addr, err := netip.ParseAddr(addrStr)
		if err != nil {
			return nil, err
		}

		rec := dns.Record{
			Type:    dns.A,
			Name:    dns.NormName(lease.Comment),
			Address: addr,
			Source:  d.id,
		}

		ok, err := d.cfg.Filter.Match(rec)
		if err != nil {
			return nil, err
		}

		if !ok {
			d.log.Debug("filter drop", "record", rec)
			continue
		}

		recs = append(recs, rec)
	}

	return recs, nil
}

func (d *DHCP) leases(ctx context.Context) ([]Lease, error) {
	return rest.Get[[]Lease](ctx, rest.Request{
		URL:  d.cfg.URL,
		Path: "/rest/ip/dhcp-server/lease",

		BasicUser: d.cfg.User,
		BasicPass: d.cfg.Password,
	})
}
