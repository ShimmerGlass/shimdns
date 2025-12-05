package netbox

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/netbox-community/go-netbox/v4"
)

const Type = "netbox"

type Netbox struct {
	log *slog.Logger
	cfg Config
	id  string

	client *netbox.APIClient
}

func New(log *slog.Logger, cfg Config, id string) (*Netbox, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	return &Netbox{
		log:    log,
		cfg:    cfg,
		id:     id,
		client: netbox.NewAPIClientFor(cfg.URL, cfg.Token),
	}, nil
}

func (n *Netbox) ID() string {
	return n.id
}

func (n *Netbox) Read(ctx context.Context) ([]dns.Record, error) {
	ctx, cancel := context.WithTimeout(ctx, n.cfg.Timeout)
	defer cancel()

	records := []dns.Record{}
	offset := int32(0)

	for {
		res, _, err := n.client.IpamAPI.IpamIpAddressesList(ctx).DnsNameEmpty(false).Offset(offset).Execute()
		if err != nil {
			return nil, fmt.Errorf("netbox api: list ipaddresses: %w", err)
		}

		if len(res.GetResults()) == 0 {
			break
		}

		for _, addr := range res.GetResults() {
			rec, err := n.addrToRecord(addr)
			if err != nil {
				n.log.Error("record from address", "addr", addr.Address, "err", err)
				continue
			}

			ok, err := n.cfg.Filter.Match(rec)
			if err != nil {
				return nil, err
			}

			if !ok {
				n.log.Debug("filter drop", "record", rec)
				continue
			}

			records = append(records, rec)
		}

		offset += int32(len(res.GetResults()))
	}

	return records, nil
}

func (n *Netbox) addrToRecord(addr netbox.IPAddress) (dns.Record, error) {
	if addr.DnsName == nil {
		return dns.Record{}, fmt.Errorf("addr has no dns name")
	}

	name := dns.NormName(*addr.DnsName)

	ip, err := netip.ParsePrefix(addr.Address)
	if err != nil {
		return dns.Record{}, err
	}

	rec := dns.Record{
		Name:    name,
		TTL:     n.cfg.TTL,
		Address: ip.Addr(),
		Source:  n.id,
	}

	if ip.Addr().Is4() {
		rec.Type = dns.A
	} else {
		rec.Type = dns.AAAA
	}

	return rec, nil
}
