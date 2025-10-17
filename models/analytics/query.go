package analytics

import (
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
)

type QueryOp string

const (
	Eq  QueryOp = "="
	Ne  QueryOp = "!="
	Gt  QueryOp = ">"
	Gte QueryOp = ">="
	Lt  QueryOp = "<"
	Lte QueryOp = "<="
	In  QueryOp = "in"
)

var ValidAnalyticsQueryOps = []QueryOp{Eq, Ne, Gt, Gte, Lt, Lte, In}

func IsValidAnalyticsQueryOp(op QueryOp) bool {
	return slices.Contains(ValidAnalyticsQueryOps, op)
}

type QueryObjectFilter struct {
	Source models.AnalyticsFieldSource `json:"source"`
	Field  string                      `json:"field"`
	Op     QueryOp                     `json:"op"`
	Values []any                       `json:"values"`
}

func (f QueryObjectFilter) Validate() error {
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

func (f QueryObjectFilter) ToPredicate(aliases ...string) (string, []any, error) {
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
