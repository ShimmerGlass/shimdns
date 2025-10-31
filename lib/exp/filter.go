package exp

import (
	"fmt"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"gopkg.in/yaml.v3"
)

type Accept struct {
	prog *Prog[bool]
}

func NewAccept(expression string) (*Accept, error) {
	program, err := Compile[bool](expression)
	if err != nil {
		return nil, err
	}

	return &Accept{prog: program}, nil
}

func (a *Accept) Match(rec dns.Record) (bool, error) {
	return a.prog.Run(rec)
}

type Reject struct {
	a *Accept
}

func NewReject(expression string) (*Reject, error) {
	a, err := NewAccept(expression)
	if err != nil {
		return nil, err
	}

	return &Reject{a: a}, nil
}

func (r *Reject) Match(rec dns.Record) (bool, error) {
	v, err := r.a.Match(rec)
	if err != nil {
		return false, err
	}

	return !v, nil
}

type Filter struct {
	accept *Accept
	reject *Reject
}

type FilterConfig struct {
	Accept string `yaml:"accept"`
	Reject string `yaml:"reject"`
}

func (f *Filter) Match(rec dns.Record) (bool, error) {
	if f.accept != nil {
		ok, err := f.accept.Match(rec)
		if err != nil {
			return false, err
		}

		if !ok {
			return false, nil
		}
	}

	if f.reject != nil {
		ok, err := f.reject.Match(rec)
		if err != nil {
			return false, err
		}

		if !ok {
			return false, nil
		}
	}

	return true, nil
}

func (f *Filter) Filter(recs []dns.Record) ([]dns.Record, error) {
	res := make([]dns.Record, 0, len(recs))

	for _, rec := range recs {
		ok, err := f.Match(rec)
		if err != nil {
			return nil, err
		}

		if ok {
			res = append(res, rec)
		}
	}

	return res, nil
}

func (f *Filter) UnmarshalYAML(value *yaml.Node) error {
	var cfg FilterConfig
	err := value.Decode(&cfg)
	if err != nil {
		return err
	}

	if cfg.Accept != "" {
		f.accept, err = NewAccept(cfg.Accept)
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}
	}

	if cfg.Reject != "" {
		f.reject, err = NewReject(cfg.Reject)
		if err != nil {
			return fmt.Errorf("reject: %w", err)
		}
	}

	return nil
}
