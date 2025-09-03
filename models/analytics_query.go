package models

import (
	"fmt"
	"slices"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
)

type AnalyticsQueryOp string

const (
	Eq  AnalyticsQueryOp = "="
	Ne                   = "!="
	Gt                   = ">"
	Gte                  = ">="
	Lt                   = "<"
	Lte                  = "<="
	In                   = "in"
)

var (
	ValidAnalyticsQueryOps = []AnalyticsQueryOp{Eq, Ne, Gt, Gte, Lt, Lte, In}
)

func IsValidAnalyticsQueryOp(op AnalyticsQueryOp) bool {
	return slices.Contains(ValidAnalyticsQueryOps, op)
}

type AnalyticsQueryObjectFilter struct {
	Field  string           `json:"field"`
	Op     AnalyticsQueryOp `json:"op"`
	Values []any            `json:"values"`
}

func (f AnalyticsQueryObjectFilter) Validate() error {
	if !IsValidAnalyticsQueryOp(f.Op) {
		return errors.Newf("unknown filter operator %s", f.Op)
	}

	if f.Op != In && len(f.Values) != 1 {
		return errors.Newf("operator %s only support one argument", f.Op)
	}
	if f.Op == In && len(f.Values) < 1 {
		return errors.Newf("operator %s requires at least one argument", f.Op)
	}

	return nil
}

func (f AnalyticsQueryObjectFilter) ToPredicate(aliases ...string) (string, []any, error) {
	alias := "main"
	if len(aliases) > 0 {
		alias = aliases[0]
	}

	switch f.Op {
	case Eq:
		return fmt.Sprintf("%s = ?", pgx.Identifier.Sanitize([]string{alias, f.Field})), f.Values, nil
	case Ne:
		return fmt.Sprintf("%s != ?", pgx.Identifier.Sanitize([]string{alias, f.Field})), f.Values, nil
	case Gt:
		return fmt.Sprintf("%s > ?", pgx.Identifier.Sanitize([]string{alias, f.Field})), f.Values, nil
	case Gte:
		return fmt.Sprintf("%s >= ?", pgx.Identifier.Sanitize([]string{alias, f.Field})), f.Values, nil
	case Lt:
		return fmt.Sprintf("%s < ?", pgx.Identifier.Sanitize([]string{alias, f.Field})), f.Values, nil
	case Lte:
		return fmt.Sprintf("%s <= ?", pgx.Identifier.Sanitize([]string{alias, f.Field})), f.Values, nil
	case In:
		return fmt.Sprintf("%s in ?", pgx.Identifier.Sanitize([]string{alias, f.Field})), f.Values, nil
	}

	return "", nil, errors.Newf("unknown filter operator %s", f.Op)
}
