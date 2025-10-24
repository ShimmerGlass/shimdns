package exp

import (
	"fmt"
	"net/netip"
	"reflect"

	"github.com/ShimmerGlass/shimdns/lib/dns"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type Prog[T any] struct {
	p *vm.Program
}

func Compile[T any](exp string, opts ...expr.Option) (*Prog[T], error) {
	var z T

	copts := []expr.Option{
		expr.Env(env{}),
		expr.AsKind(reflect.TypeOf(z).Kind()),
	}

	copts = append(copts, funcs...)
	copts = append(copts, opts...)

	p, err := expr.Compile(exp, copts...)
	if err != nil {
		return nil, err
	}

	return &Prog[T]{p: p}, nil
}

func (p *Prog[T]) Run(rec dns.Record) (T, error) {
	var z T

	res, err := expr.Run(p.p, newEnv(rec))
	if err != nil {
		return z, err
	}

	tres, ok := res.(T)
	if !ok {
		return z, fmt.Errorf("unexpected return type %T, wanted %T", res, z)
	}

	return tres, nil
}

type env struct {
	Record dns.Record `expr:"record"`
}

func newEnv(rec dns.Record) env {
	return env{
		Record: rec,
	}
}

var funcs = []expr.Option{
	expr.Function(
		"subnetContains",
		func(params ...any) (any, error) {
			var err error
			var subnet netip.Prefix
			var addr netip.Addr

			switch p0 := params[0].(type) {
			case string:
				subnet, err = netip.ParsePrefix(p0)
				if err != nil {
					return nil, err
				}
			case netip.Prefix:
				subnet = p0
			}

			switch p1 := params[1].(type) {
			case string:
				addr, err = netip.ParseAddr(p1)
				if err != nil {
					return nil, err
				}
			case netip.Addr:
				addr = p1
			}

			return subnet.Contains(addr), nil
		},
		new(func(string, string) bool),
		new(func(netip.Prefix, string) bool),
		new(func(netip.Prefix, netip.Addr) bool),
		new(func(string, netip.Addr) bool),
	),
	expr.Function(
		"ip",
		func(params ...any) (any, error) {
			return netip.ParseAddr(params[0].(string))
		},
		new(func(string) netip.Addr),
	),
	expr.Function(
		"subnet",
		func(params ...any) (any, error) {
			return netip.ParsePrefix(params[0].(string))
		},
		new(func(string) netip.Prefix),
	),
}
