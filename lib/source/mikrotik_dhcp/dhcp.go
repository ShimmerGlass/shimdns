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
}

func New(log *slog.Logger, cfg Config) (*DHCP, error) {
	if cfg.Name == "" {
		cfg.Name = cfg.URL
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	return &DHCP{
		log: log.With("source", Type, "source_name", cfg.Name),
		cfg: cfg,
	}, nil
}

func (h *DHCP) Type() string {
	return Type
}

func (h *DHCP) Name() string {
	return h.cfg.Name
}

func (h *DHCP) Read(ctx context.Context) ([]dns.Record, error) {
	ctx, cancel := context.WithTimeout(ctx, h.cfg.Timeout)
	defer cancel()

	leases, err := h.leases(ctx)
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
			Type:       dns.A,
			Name:       dns.NormName(lease.Comment),
			Address:    addr,
			Source:     Type,
			SourceName: h.cfg.Name,
		}

		ok, err := h.cfg.Filter.Match(rec)
		if err != nil {
			return nil, err
		}

		if !ok {
			h.log.Debug("filter drop", "record", rec)
			continue
		}

		recs = append(recs, rec)
	}

	return recs, nil
}

func (h *DHCP) leases(ctx context.Context) ([]Lease, error) {
	return rest.Get[[]Lease](ctx, rest.Request{
		URL:  h.cfg.URL,
		Path: "/rest/ip/dhcp-server/lease",

		BasicUser: h.cfg.User,
		BasicPass: h.cfg.Password,
	})
}
