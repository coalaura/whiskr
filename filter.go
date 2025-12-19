package main

import (
	"fmt"
	"strconv"
	"strings"
)

type Operator string

const (
	OpLess       Operator = "<"
	OpGreater    Operator = ">"
	OpEqual      Operator = "="
	OpContains   Operator = "~"
	OpStartsWith Operator = "^"
	OpEndsWith   Operator = "$"
)

type FieldDef struct {
	Type      string
	AllowedOp map[Operator]bool
	Matcher   func(m *Model, op Operator, val string) bool
}

var fieldRegistry = map[string]FieldDef{
	"price": {
		Type:      "number",
		AllowedOp: map[Operator]bool{OpLess: true, OpGreater: true, OpEqual: true},
		Matcher: func(m *Model, op Operator, val string) bool {
			fv, _ := strconv.ParseFloat(val, 64)

			return compare(max(m.Pricing.Input, m.Pricing.Output), fv, op)
		},
	},
	"name": {
		Type:      "string",
		AllowedOp: map[Operator]bool{OpEqual: true, OpContains: true, OpStartsWith: true, OpEndsWith: true},
		Matcher: func(m *Model, op Operator, val string) bool {
			return compareString(strings.ToLower(m.Name), val, op)
		},
	},
	"id": {
		Type:      "string",
		AllowedOp: map[Operator]bool{OpEqual: true, OpContains: true},
		Matcher: func(m *Model, op Operator, val string) bool {
			return compareString(m.ID, val, op)
		},
	},
}

type Filter struct {
	field    string
	operator Operator
	rawValue string
}

type FilterList []*Filter

func (f FilterList) Match(md *Model) bool {
	for _, filter := range f {
		if filter.Match(md) {
			return true
		}
	}

	return false
}

func (f *Filter) Match(md *Model) bool {
	def, ok := fieldRegistry[f.field]
	if !ok {
		return false
	}

	return def.Matcher(md, f.operator, f.rawValue)
}

func ParseFilters(filterStrs string) (FilterList, error) {
	if strings.TrimSpace(filterStrs) == "" {
		return nil, nil
	}

	parts := strings.Split(filterStrs, ",")

	filters := make(FilterList, 0, len(parts))

	for _, part := range parts {
		f, err := ParseFilter(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}

		filters = append(filters, f)
	}

	return filters, nil
}

func ParseFilter(input string) (*Filter, error) {
	idx := strings.IndexAny(input, "<>=~^$")
	if idx <= 0 {
		return nil, fmt.Errorf("invalid filter format: %q", input)
	}

	field := strings.ToLower(strings.TrimSpace(input[:idx]))
	op := Operator(input[idx : idx+1])
	val := strings.ToLower(strings.TrimSpace(input[idx+1:]))

	def, ok := fieldRegistry[field]
	if !ok {
		return nil, fmt.Errorf("unknown field: %s", field)
	}

	if !def.AllowedOp[op] {
		return nil, fmt.Errorf("operator %q not allowed for field %q", op, field)
	}

	if def.Type == "number" {
		_, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number value: %q", val)
		}
	}

	return &Filter{
		field:    field,
		operator: op,
		rawValue: val,
	}, nil
}

func compare[T float64 | int64](a, b T, op Operator) bool {
	switch op {
	case OpLess:
		return a < b
	case OpGreater:
		return a > b
	case OpEqual:
		return a == b
	default:
		return false
	}
}

func compareString(a, b string, op Operator) bool {
	switch op {
	case OpEqual:
		return a == b
	case OpContains:
		return strings.Contains(a, b)
	case OpStartsWith:
		return strings.HasPrefix(a, b)
	case OpEndsWith:
		return strings.HasSuffix(a, b)
	default:
		return false
	}
}
