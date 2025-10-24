package autoptr

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"

	"github.com/ShimmerGlass/shimdns/lib/dns"
)

type PTR struct {
	log *slog.Logger
	cfg Config
}

func New(log *slog.Logger, cfg Config) (*PTR, error) {
	return &PTR{
		log: log.With("modifier", "autoptr"),
		cfg: cfg,
	}, nil
}

func (p *PTR) Modify(ctx context.Context, records []dns.Record) ([]dns.Record, error) {
	present := map[netip.Addr]struct{}{}
	for _, rec := range records {
		if rec.Type != dns.PTR {
			continue
		}

		addr, err := ptrToAddr(rec.Name)
		if err != nil {
			return nil, fmt.Errorf("ptr to addr: %s: %w", rec.Name, err)
		}

		present[addr] = struct{}{}
	}

	for _, rec := range records {
		ok, err := p.cfg.Filter.Match(rec)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		if rec.Type != dns.A && rec.Type != dns.AAAA {
			continue
		}

		if _, ok := present[rec.Address]; ok {
			continue
		}

		ptr, err := addrToPTR(rec.Address)
		if err != nil {
			return nil, fmt.Errorf("addr to ptr: %s: %w", rec.Address, err)
		}

		records = append(records, dns.Record{
			Type:   dns.PTR,
			Name:   ptr,
			Ptr:    rec.Name,
			Source: "autoptr",
		})

		present[rec.Address] = struct{}{}
	}

	return records, nil
}

func addrToPTR(addr netip.Addr) (string, error) {
	if !addr.IsValid() {
		return "", fmt.Errorf("invalid address")
	}

	if addr.Is4() {
		ip := addr.As4()
		// Reverse octets for IPv4
		return fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa.", ip[3], ip[2], ip[1], ip[0]), nil
	}

	if addr.Is6() {
		ip := addr.As16()
		// Reverse nibbles for IPv6
		var nibbles []string
		for i := len(ip) - 1; i >= 0; i-- {
			nibbles = append(nibbles, fmt.Sprintf("%x.%x", ip[i]&0x0F, ip[i]>>4))
		}

		return strings.Join(nibbles, ".") + ".ip6.arpa.", nil
	}

	return "", fmt.Errorf("unsupported address type")
}

func ptrToAddr(ptr string) (netip.Addr, error) {
	ptr = strings.TrimSuffix(ptr, ".")
	ptr = strings.ToLower(ptr)

	switch {
	case strings.HasSuffix(ptr, ".in-addr.arpa"):
		// IPv4 reverse: d.c.b.a.in-addr.arpa
		trimmed := strings.TrimSuffix(ptr, ".in-addr.arpa")
		parts := strings.Split(trimmed, ".")
		if len(parts) != 4 {
			return netip.Addr{}, fmt.Errorf("invalid IPv4 PTR: %q", ptr)
		}

		// Reverse order back to a.b.c.d
		addrStr := fmt.Sprintf("%s.%s.%s.%s",
			parts[3], parts[2], parts[1], parts[0])

		return netip.ParseAddr(addrStr)

	case strings.HasSuffix(ptr, ".ip6.arpa"):
		// IPv6 reverse: each nibble reversed
		trimmed := strings.TrimSuffix(ptr, ".ip6.arpa")
		parts := strings.Split(trimmed, ".")
		if len(parts) != 32 {
			return netip.Addr{}, fmt.Errorf("invalid IPv6 PTR: %q", ptr)
		}

		// Reverse nibble order and group into hex string
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		ipHex := strings.Join(parts, "")
		// Insert colons every 4 nibbles
		var blocks []string
		for i := 0; i < 32; i += 4 {
			blocks = append(blocks, ipHex[i:i+4])
		}

		ipStr := strings.Join(blocks, ":")
		return netip.ParseAddr(ipStr)

	default:
		return netip.Addr{}, fmt.Errorf("unknown PTR suffix: %q", ptr)
	}
}
