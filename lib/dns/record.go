package dns

import (
	"fmt"
	"log/slog"
	"net/netip"
	"strings"

	"github.com/samber/lo"
)

const (
	A     = "A"
	AAAA  = "AAAA"
	PTR   = "PTR"
	CNAME = "CNAME"
	SRV   = "SRV"
	MX    = "MX"
	HTTPS = "HTTPS"
)

const (
	AlpnHTTP11 = "http/1.1"
	AlpnHTTP2  = "h2"
	AlpnHTTP3  = "h3"
)

type Record struct {
	Type string `json:"type" expr:"type" yaml:"type"`
	Name string `json:"name" expr:"name" yaml:"name"`

	Source string `json:"source" expr:"source" yaml:"source"`

	// for A & AAAA
	Address netip.Addr `json:"address,omitempty" expr:"address" yaml:"address"`
	// for PTR
	Ptr string `json:"ptr,omitempty" expr:"ptr" yaml:"ptr"`
	// for CNAME, SRV, HTTPS
	Target string `json:"target,omitempty" expr:"target" yaml:"target"`
	// for SRV, HTTP
	Priority uint16 `json:"priority,omitempty" expr:"priority" yaml:"priority"`
	// for SRV
	Weight uint16 `json:"weight,omitempty" expr:"weight" yaml:"weight"`
	Port   uint16 `json:"port,omitempty" expr:"port" yaml:"port"`
	// for MX
	Preference uint16 `json:"preference,omitempty" expr:"preference" yaml:"preference"`
	Mx         string `json:"mx,omitempty" expr:"mx" yaml:"mx"`
	// for HTTPS
	Alpn     []string     `json:"alpn,omitempty" expr:"alpn" yaml:"alpn"`
	IPv4Hint []netip.Addr `json:"ipv4_hint,omitempty" expr:"ipv4_hint" yaml:"ipv4_hint"`
	IPv6Hint []netip.Addr `json:"ipv6_hint,omitempty" expr:"ipv6_hint" yaml:"ipv6_hint"`
}

func (r Record) String() string {
	return fmt.Sprintf("%s IN %s %s", r.Name, r.Type, r.RData())
}

func (r Record) RData() string {
	switch r.Type {
	case A, AAAA:
		return r.Address.String()

	case PTR:
		return r.Ptr

	case CNAME:
		return r.Target

	case SRV:
		return fmt.Sprintf("%d %d %d %s", r.Priority, r.Weight, r.Port, r.Target)

	case MX:
		return fmt.Sprintf("%d %s", r.Preference, r.Mx)

	case HTTPS:
		v := fmt.Sprintf("%d %s", r.Priority, r.Target)
		if len(r.Alpn) > 0 {
			v += fmt.Sprintf(" alpn=%s", strings.Join(r.Alpn, ","))
		}
		if len(r.IPv4Hint) > 0 {
			v += fmt.Sprintf(" ipv4hint=%s", strings.Join(lo.Map(r.IPv4Hint, func(a netip.Addr, _ int) string {
				return a.String()
			}), ","))
		}
		if len(r.IPv6Hint) > 0 {
			v += fmt.Sprintf(" ipv6hint=%s", strings.Join(lo.Map(r.IPv6Hint, func(a netip.Addr, _ int) string {
				return a.String()
			}), ","))
		}
		return v

	default:
		panic(fmt.Sprintf("record type %q not handled", r.Type))
	}
}

func (r Record) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("name", r.Name),
		slog.String("type", r.Type),
		slog.String("source", r.Source),
	}

	switch r.Type {
	case A, AAAA:
		attrs = append(attrs, slog.String("address", r.Address.String()))

	case PTR:
		attrs = append(attrs, slog.String("ptr", r.Ptr))

	case CNAME:
		attrs = append(attrs, slog.String("target", r.Target))

	case SRV:
		attrs = append(attrs,
			slog.String("target", r.Target),
			slog.Int("priority", int(r.Priority)),
			slog.Int("weight", int(r.Weight)),
			slog.Int("port", int(r.Port)),
		)

	case MX:
		attrs = append(attrs,
			slog.String("mx", r.Mx),
			slog.Int("preference", int(r.Preference)),
		)

	case HTTPS:
		attrs = append(attrs,
			slog.String("target", r.Target),
			slog.Int("priority", int(r.Priority)),
			slog.Any("alpn", r.Alpn),
			slog.Any("ipv4_hint", r.IPv4Hint),
			slog.Any("ipv6_hint", r.IPv6Hint),
		)

	default:
		panic(fmt.Sprintf("record type %q not handled", r.Type))
	}

	return slog.GroupValue(attrs...)
}

type Records struct {
	Records []Record `json:"records" yaml:"records"`
}
