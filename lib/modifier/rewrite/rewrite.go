package rewrite

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/ShimmerGlass/shimdns/lib/exp"
	"github.com/samber/lo"
)

const Type = "rewrite"

type Rewrite struct {
	log *slog.Logger
	cfg Config
	id  string

	set map[string]*exp.Prog[any]
}

func New(log *slog.Logger, cfg Config, id string) (*Rewrite, error) {
	r := &Rewrite{
		log: log,
		cfg: cfg,
		id:  id,

		set: map[string]*exp.Prog[any]{},
	}

	err := r.initExprs()
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Rewrite) ID() string {
	return r.id
}

func (r *Rewrite) initExprs() error {
	fieldMap := map[string]string{}

	rt := reflect.TypeOf(dns.Record{})
	for i := range rt.NumField() {
		field := rt.Field(i)
		fieldMap[field.Tag.Get("expr")] = field.Name
	}

	for k, v := range r.cfg.Set {
		fieldName, ok := fieldMap[k]
		if !ok {
			return fmt.Errorf("invalid set field %q, accepted fields are %v", k, lo.Keys(fieldMap))
		}

		expr, err := exp.CompileAny(v)
		if err != nil {
			return fmt.Errorf("set.%s: %w", k, err)
		}

		r.set[fieldName] = expr
	}

	return nil
}

func (r *Rewrite) Modify(ctx context.Context, records []dns.Record) ([]dns.Record, error) {
	for i, rec := range records {
		ok, err := r.cfg.Filter.Match(rec)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		rec, err := r.modifyRecord(ctx, rec)
		if err != nil {
			return nil, err
		}

		records[i] = rec
	}

	return records, nil
}

func (r *Rewrite) modifyRecord(ctx context.Context, rec dns.Record) (dns.Record, error) {
	recVal := reflect.ValueOf(&rec)

	for field, expr := range r.set {
		v, err := expr.Run(rec)
		if err != nil {
			return rec, fmt.Errorf("%s: %w", field, err)
		}

		field := recVal.Elem().FieldByName(field)
		switch field.Kind() {
		case reflect.Slice:
			nv := reflect.New(field.Type()).Elem()
			for _, el := range v.([]any) {
				nv = reflect.Append(nv, reflect.ValueOf(el))
			}
			field.Set(nv)

		default:
			field.Set(reflect.ValueOf(v))
		}
	}

	return rec, nil
}
