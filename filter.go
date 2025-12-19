package main

import (
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type Filters struct {
	program *vm.Program
}

type FilterModel struct {
	Slug  string  `expr:"slug"`
	Name  string  `expr:"name"`
	Price float64 `expr:"price"`
}

func (f *Filters) Match(md *Model) (bool, error) {
	match, err := expr.Run(f.program, FilterModel{
		Slug:  md.ID,
		Name:  md.Name,
		Price: max(md.Pricing.Input, md.Pricing.Output),
	})

	if err != nil {
		return false, err
	}

	return match.(bool), nil
}

func ParseFilters(query string) (*Filters, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	query = strings.ReplaceAll(query, "~", " contains ")
	query = strings.ReplaceAll(query, "^", " has_prefix ")
	query = strings.ReplaceAll(query, "$", " has_suffix ")

	options := []expr.Option{
		expr.AsBool(),
		expr.Env(FilterModel{}),

		expr.Function("contains",
			func(params ...any) (any, error) {
				p1 := strings.ToLower(params[0].(string))
				p2 := strings.ToLower(params[0].(string))

				return strings.Contains(p1, p2), nil
			},
			new(func(string, string) bool),
		),
		expr.Function("has_prefix",
			func(params ...any) (any, error) {
				p1 := strings.ToLower(params[0].(string))
				p2 := strings.ToLower(params[0].(string))

				return strings.HasPrefix(p1, p2), nil
			},
			new(func(string, string) bool),
		),
		expr.Function("has_suffix",
			func(params ...any) (any, error) {
				p1 := strings.ToLower(params[0].(string))
				p2 := strings.ToLower(params[0].(string))

				return strings.HasSuffix(p1, p2), nil
			},
			new(func(string, string) bool),
		),
	}

	program, err := expr.Compile(query, options...)
	if err != nil {
		return nil, err
	}

	return &Filters{
		program: program,
	}, nil
}
