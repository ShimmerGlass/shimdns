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

// modifyRecord evaluates all the set expressions against the original record
// before assigning any field, so that the result does not depend on the order
// in which fields are set.
func (r *Rewrite) modifyRecord(ctx context.Context, rec dns.Record) (dns.Record, error) {
	values := make(map[string]any, len(r.set))

	for field, expr := range r.set {
		v, err := expr.Run(rec)
		if err != nil {
			return rec, fmt.Errorf("%s: %w", field, err)
		}

		values[field] = v
	}

	recVal := reflect.ValueOf(&rec).Elem()

	for field, v := range values {
		nv, err := convertValue(v, recVal.FieldByName(field).Type())
		if err != nil {
			return rec, fmt.Errorf("%s: %w", field, err)
		}

		recVal.FieldByName(field).Set(nv)
	}

	return rec, nil
}

func convertValue(v any, t reflect.Type) (reflect.Value, error) {
	if v == nil {
		return reflect.Value{}, fmt.Errorf("cannot assign nil to %s", t)
	}

	rv := reflect.ValueOf(v)
	if rv.Type().AssignableTo(t) {
		return rv, nil
	}

	if t.Kind() == reflect.Slice && rv.Kind() == reflect.Slice {
		nv := reflect.MakeSlice(t, 0, rv.Len())
		for i := range rv.Len() {
			el, err := convertValue(rv.Index(i).Interface(), t.Elem())
			if err != nil {
				return reflect.Value{}, fmt.Errorf("element %d: %w", i, err)
			}
			nv = reflect.Append(nv, el)
		}

		return nv, nil
	}

	return reflect.Value{}, fmt.Errorf("cannot assign %T to %s", v, t)
}
