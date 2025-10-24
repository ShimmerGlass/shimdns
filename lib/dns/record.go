package dns

import (
	"fmt"
	"log/slog"
	"net/netip"
)

type Type string

const (
	A     Type = "A"
	AAAA  Type = "AAAA"
	PTR   Type = "PTR"
	CNAME Type = "CNAME"
	SRV   Type = "SRV"
	MX    Type = "MX"
)

type Record struct {
	Type Type   `json:"type" expr:"type" yaml:"type"`
	Name string `json:"name" expr:"name" yaml:"name"`

	Source     string `json:"source" expr:"source" yaml:"source"`
	SourceName string `json:"source_name" expr:"source_name" yaml:"source_name"`

	// for A & AAAA
	Address netip.Addr `json:"address,omitempty" expr:"address" yaml:"address"`
	// for PTR
	Ptr string `json:"ptr,omitempty" expr:"ptr" yaml:"ptr"`
	// for CNAME & SRV
	Target string `json:"target,omitempty" expr:"target" yaml:"target"`
	// for SRV
	Priority uint16 `json:"priority,omitempty" expr:"priority" yaml:"priority"`
	Weight   uint16 `json:"weight,omitempty" expr:"weight" yaml:"weight"`
	Port     uint16 `json:"port,omitempty" expr:"port" yaml:"port"`
	// for MX
	Preference uint16 `json:"preference,omitempty" expr:"preference" yaml:"preference"`
	Mx         string `json:"mx,omitempty" expr:"mx" yaml:"mx"`
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

	default:
		panic(fmt.Sprintf("record type %q not handled", r.Type))
	}
}

func (r Record) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("name", r.Name),
		slog.String("type", string(r.Type)),
		slog.String("source", r.Source),
	}

	if r.SourceName != "" {
		attrs = append(attrs, slog.String("source_name", r.SourceName))
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

	default:
		panic(fmt.Sprintf("record type %q not handled", r.Type))
	}

	return slog.GroupValue(attrs...)
}

type Records struct {
	Records []Record `json:"records" yaml:"records"`
}
