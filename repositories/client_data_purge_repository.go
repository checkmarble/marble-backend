package repositories

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
)

type ClientDataPurgeRepository interface {
	DeleteStaleRowsBefore(ctx context.Context, exec Executor, table string, before time.Time) (int, error)
	DeleteActiveRowsBefore(ctx context.Context, exec Executor, table string, before time.Time) (int, error)
}

func (repo *MarbleDbRepository) DeleteStaleRowsBefore(ctx context.Context, exec Executor, table string, before time.Time) (int, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return 0, err
	}

	tableName := sanitizedTableName(exec, table)

	cte := WithCtes("objects", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
		return b.
			Select("id").
			From(tableName).
			Where("valid_until < ?", before.UTC()).
			OrderBy("valid_until").
			Limit(5000)
	})

	sql := NewQueryBuilder().
		Delete(sanitizedTableName(exec, table) + " t").
		PrefixExpr(cte).
		Suffix("using objects where t.id = objects.id")

	query, args, err := sql.ToSql()
	if err != nil {
		return 0, err
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	return int(tag.RowsAffected()), nil
}

func (repo *MarbleDbRepository) DeleteActiveRowsBefore(ctx context.Context, exec Executor, table string, before time.Time) (int, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return 0, err
	}

	tableName := sanitizedTableName(exec, table)

	cte := WithCtes("objects", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
		return b.
			Select("id").
			From(tableName).
			Where("valid_from < ? and valid_until = 'infinity'", before.UTC()).
			OrderBy("valid_until").
			Limit(5000)
	})

	sql := NewQueryBuilder().
		Delete(sanitizedTableName(exec, table) + " t").
		PrefixExpr(cte).
		Suffix("using objects where t.id = objects.id")

	query, args, err := sql.ToSql()
	if err != nil {
		return 0, err
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	return int(tag.RowsAffected()), nil
}
