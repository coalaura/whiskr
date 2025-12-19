package main

import (
	"regexp"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type FilterPatcher struct{}

type Filters struct {
	program *vm.Program
}

type FilterModel struct {
	Slug    string   `expr:"slug"`
	Name    string   `expr:"name"`
	Price   float64  `expr:"price"`
	Tags    []string `expr:"tags"`
	Created int64    `expr:"created"`
}

func (f *Filters) Match(md *Model) (bool, error) {
	match, err := expr.Run(f.program, FilterModel{
		Slug:    md.ID,
		Name:    md.Name,
		Price:   max(md.Pricing.Input, md.Pricing.Output),
		Tags:    md.Tags,
		Created: md.Created,
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

	containsRgx := regexp.MustCompile(`(\w+)\s*~\s*('[^']+'|"[^"]+"|\w+)`)
	query = containsRgx.ReplaceAllString(query, "_contains($1, $2)")

	prefixRgx := regexp.MustCompile(`(\w+)\s*\^\s*('[^']+'|"[^"]+"|\w+)`)
	query = prefixRgx.ReplaceAllString(query, "_has_prefix($1, $2)")

	suffixRgx := regexp.MustCompile(`(\w+)\s*\$\s*('[^']+'|"[^"]+"|\w+)`)
	query = suffixRgx.ReplaceAllString(query, "_has_suffix($1, $2)")

	options := []expr.Option{
		expr.AsBool(),
		expr.Env(FilterModel{}),

		expr.Function("_contains",
			func(params ...any) (any, error) {
				search := strings.ToLower(params[1].(string))

				switch val := params[0].(type) {
				case string:
					val = strings.ToLower(val)

					return strings.Contains(val, search), nil
				case []string:
					for _, entry := range val {
						if strings.ToLower(entry) == search {
							return true, nil
						}
					}

					return false, nil
				}

				return false, nil
			},
			new(func(any, string) bool),
		),

		expr.Function("_has_prefix",
			func(params ...any) (any, error) {
				p1 := strings.ToLower(params[0].(string))
				p2 := strings.ToLower(params[1].(string))

				return strings.HasPrefix(p1, p2), nil
			},
			new(func(string, string) bool),
		),

		expr.Function("_has_suffix",
			func(params ...any) (any, error) {
				p1 := strings.ToLower(params[0].(string))
				p2 := strings.ToLower(params[1].(string))

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
