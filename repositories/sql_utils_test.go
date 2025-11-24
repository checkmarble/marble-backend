package repositories

import (
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/stretchr/testify/assert"
)

func TestSingleCte(t *testing.T) {
	ctes := WithCtes("a", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
		return b.Select("1")
	})

	q := NewQueryBuilder().
		Select("2").
		PrefixExpr(ctes)

	sql, _, _ := q.ToSql()

	assert.Equal(t, `with "a" as ( SELECT 1 ) SELECT 2`, sql)
}

func TestMultipleCtes(t *testing.T) {
	ctes := WithCtes("a", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
		return b.Select("1")
	}).
		With("b", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
			return b.Select("2")
		})

	q := NewQueryBuilder().
		Select("3").
		PrefixExpr(ctes)

	sql, _, _ := q.ToSql()

	assert.Equal(t, `with "a" as ( SELECT 1 ) , "b" as ( SELECT 2 ) SELECT 3`, sql)
}
