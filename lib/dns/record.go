package dns

import (
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
)

const (
	A     = "A"
	AAAA  = "AAAA"
	PTR   = "PTR"
	CNAME = "CNAME"
	SRV   = "SRV"
	MX    = "MX"
	HTTPS = "HTTPS"
	ALIAS = "ALIAS"
	TXT   = "TXT"
	CAA   = "CAA"
)

// Types lists the supported record types
var Types = []string{A, AAAA, PTR, CNAME, SRV, MX, HTTPS, ALIAS, TXT, CAA}

func ValidType(t string) bool {
	return slices.Contains(Types, t)
}

const (
	AlpnHTTP11 = "http/1.1"
	AlpnHTTP2  = "h2"
	AlpnHTTP3  = "h3"
)

type Record struct {
	Type string `json:"type" expr:"type" yaml:"type"`
	Name string `json:"name" expr:"name" yaml:"name"`
	TTL  int    `json:"ttl" expr:"ttl" yaml:"ttl"`

	Source string `json:"source" expr:"source" yaml:"source"`

	// for A & AAAA
	Address netip.Addr `json:"address,omitempty" expr:"address" yaml:"address"`
	// for PTR
	Ptr string `json:"ptr,omitempty" expr:"ptr" yaml:"ptr"`
	// for CNAME, ALIAS, SRV, HTTPS
	Target string `json:"target,omitempty" expr:"target" yaml:"target"`
	// for SRV, HTTP
	Priority int `json:"priority,omitempty" expr:"priority" yaml:"priority"`
	// for SRV
	Weight int `json:"weight,omitempty" expr:"weight" yaml:"weight"`
	Port   int `json:"port,omitempty" expr:"port" yaml:"port"`
	// for MX
	Preference int    `json:"preference,omitempty" expr:"preference" yaml:"preference"`
	Mx         string `json:"mx,omitempty" expr:"mx" yaml:"mx"`
	// for HTTPS
	Alpn []string `json:"alpn,omitempty" expr:"alpn" yaml:"alpn"`
	// for TXT
	Text string `json:"text,omitempty" expr:"text" yaml:"text"`
	// for CAA
	Flags int    `json:"flags,omitempty" expr:"flags" yaml:"flags"`
	Tag   string `json:"tag,omitempty" expr:"tag" yaml:"tag"`
	Value string `json:"value,omitempty" expr:"value" yaml:"value"`
}

// Quote wraps a character-string in double quotes, escaping `\` and `"`.
func Quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// TxtChunks splits a TXT value into the character-strings of at most 255
// bytes a record is made of.
func TxtChunks(s string) []string {
	if s == "" {
		return []string{""}
	}

	var chunks []string
	for len(s) > 255 {
		chunks = append(chunks, s[:255])
		s = s[255:]
	}

	return append(chunks, s)
}

func (r Record) String() string {
	return fmt.Sprintf("%s %d IN %s %s", r.Name, r.TTL, r.Type, r.RData())
}

func (r Record) RData() string {
	switch r.Type {
	case A, AAAA:
		return r.Address.String()

	case PTR:
		return r.Ptr

	case CNAME, ALIAS:
		return r.Target

	case TXT:
		return r.Text

	case CAA:
		return fmt.Sprintf("%d %s %s", r.Flags, r.Tag, Quote(r.Value))

	case SRV:
		return fmt.Sprintf("%d %d %d %s", r.Priority, r.Weight, r.Port, r.Target)

	case MX:
		return fmt.Sprintf("%d %s", r.Preference, r.Mx)

	case HTTPS:
		v := fmt.Sprintf("%d %s", r.Priority, r.Target)
		if len(r.Alpn) > 0 {
			v += fmt.Sprintf(" alpn=%s", strings.Join(r.Alpn, ","))
		}
		return v

	default:
		return ""
	}
}

func (r Record) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("name", r.Name),
		slog.String("type", r.Type),
		slog.Int("ttl", int(r.TTL)),
		slog.String("source", r.Source),
	}

	switch r.Type {
	case A, AAAA:
		attrs = append(attrs, slog.String("address", r.Address.String()))

	case PTR:
		attrs = append(attrs, slog.String("ptr", r.Ptr))

	case CNAME, ALIAS:
		attrs = append(attrs, slog.String("target", r.Target))

	case TXT:
		attrs = append(attrs, slog.String("text", r.Text))

	case CAA:
		attrs = append(attrs,
			slog.Int("flags", r.Flags),
			slog.String("tag", r.Tag),
			slog.String("value", r.Value),
		)

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
		)
	}

	return slog.GroupValue(attrs...)
}

type Records struct {
	Records []Record `json:"records" yaml:"records"`
}
