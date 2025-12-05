package gandi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/samber/lo"
)

const Type = "gandi"

type Gandi struct {
	log *slog.Logger
	cfg Config
	id  string

	httpClient *http.Client
}

func New(log *slog.Logger, cfg Config, id string) (*Gandi, error) {
	return &Gandi{
		log: log,
		id:  id,
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: lo.Ternary(cfg.Timeout > 0, cfg.Timeout, 5*time.Second),
		},
	}, nil
}

func (g *Gandi) ID() string {
	return g.id
}

func (g *Gandi) Write(ctx context.Context, recs []dns.Record) error {
	recs, err := g.cfg.Filter.Filter(recs)
	if err != nil {
		return err
	}

	for _, domain := range g.cfg.Domains {
		err := g.updateDomain(domain, recs)
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *Gandi) updateDomain(domain string, recs []dns.Record) error {
	wanted := g.buildDomain(domain, recs)
	body := &bytes.Buffer{}
	err := json.NewEncoder(body).Encode(Records{Items: wanted})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		http.MethodPut,
		lo.Must(url.JoinPath("https://api.gandi.net/v5/livedns/domains", domain, "records")),
		body,
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+g.cfg.PersonalAccessToken)

	res, err := g.httpClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("update records: invalid status code %d", res.StatusCode)
	}

	return nil
}

type recKey struct {
	Name string
	Type string
}

func (g *Gandi) buildDomain(domain string, recs []dns.Record) []*DomainRecord {
	domain = dns.NormName(domain)
	grecs := map[recKey]*DomainRecord{}

	for _, rec := range recs {
		if !dns.SubdomainOf(rec.Name, domain) && rec.Name != domain {
			continue
		}

		var name string
		if rec.Name == domain {
			name = "@"
		} else {
			name = dns.RelativeTo(rec.Name, domain)
		}

		k := recKey{Name: name, Type: rec.Type}

		grec, ok := grecs[k]
		if !ok {
			grec = &DomainRecord{
				RrsetType: rec.Type,
				RrsetName: dns.RelativeTo(rec.Name, domain),
				RrsetTTL:  max(300, rec.TTL),
			}
			grecs[k] = grec
		}

		grec.RrsetValues = append(grec.RrsetValues, recRValue(rec))
	}

	return lo.Values(grecs)
}

func recRValue(r dns.Record) string {
	switch r.Type {
	case dns.HTTPS:
		v := fmt.Sprintf("%d ", r.Priority)
		if r.Target == "." {
			v += r.Name
		} else {
			v += r.Target
		}

		params := []string{}
		if len(r.Alpn) > 0 {
			params = append(params, fmt.Sprintf(`alpn="%s"`, strings.Join(r.Alpn, ",")))
		}

		if len(params) > 0 {
			v += " " + strings.Join(params, " ")
		}

		return v

	default:
		return r.RData()
	}
}
