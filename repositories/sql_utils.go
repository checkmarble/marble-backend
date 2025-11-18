package repositories

import (
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type cte struct {
	name  string
	query squirrel.SelectBuilder
}

type QueryCte struct {
	builder func() squirrel.StatementBuilderType
	queries []cte
}

func WithCtes(name string, cb func(b squirrel.StatementBuilderType) squirrel.SelectBuilder) *QueryCte {
	ctes := &QueryCte{
		builder: NewQueryBuilder,
	}

	return ctes.With(name, cb)
}

func WithCtesRaw(name string, cb func(b squirrel.StatementBuilderType) squirrel.SelectBuilder) *QueryCte {
	ctes := &QueryCte{
		builder: func() squirrel.StatementBuilderType {
			return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
		},
	}

	return ctes.With(name, cb)
}

func (q *QueryCte) With(name string, cb func(b squirrel.StatementBuilderType) squirrel.SelectBuilder) *QueryCte {
	q.queries = append(q.queries, cte{
		name:  pgx.Identifier.Sanitize([]string{name}),
		query: cb(q.builder()),
	})

	return q
}

func (q *QueryCte) ToSql() (string, []any, error) {
	var out squirrel.SelectBuilder

	for idx, cte := range q.queries {
		if idx == 0 {
			cte.query = cte.query.Prefix("with")
		}

		cte.query = cte.query.Prefix(fmt.Sprintf("%s as (", cte.name)).Suffix(")")

		if idx < len(q.queries)-1 {
			cte.query = cte.query.Suffix(",")
		}

		switch idx {
		case 0:
			out = cte.query
		default:
			out = out.SuffixExpr(cte.query)
		}
	}

	return out.ToSql()
}
