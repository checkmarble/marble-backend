package executor_factory

import (
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"gotest.tools/v3/assert"
)

func TestDifferentSeparateYears(t *testing.T) {
	f := AnalyticsExecutorFactory{}

	start := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, 10, 3, 0, 0, 0, 0, time.UTC)

	q := squirrel.Select("*").From("t")
	q = f.BuildPushdownFilter(q, "orgid", start, end, "accounts")

	sql, args, err := q.ToSql()

	assert.NilError(t, err)
	assert.Equal(t, `SELECT * FROM t WHERE "main"."trigger_object_type" = ? AND "main"."year" in ? AND (("main"."year" = ? AND "main"."month" between ? and 12) OR ("main"."year" = ? AND "main"."month" between 1 and ?))`, sql)
	assert.DeepEqual(t, []any{"accounts", []int{2025, 2026}, 2024, time.March, 2027, time.October}, args)
}

func TestSameYears(t *testing.T) {
	f := AnalyticsExecutorFactory{}

	start := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 10, 3, 0, 0, 0, 0, time.UTC)

	q := squirrel.Select("*").From("t")
	q = f.BuildPushdownFilter(q, "orgid", start, end, "accounts")

	sql, args, err := q.ToSql()

	assert.NilError(t, err)
	assert.Equal(t, `SELECT * FROM t WHERE "main"."trigger_object_type" = ? AND "main"."year" = ? and "main"."month" between ? and ?`, sql)
	assert.DeepEqual(t, []any{"accounts", 2024, time.March, time.October}, args)
}

func TestSameMonths(t *testing.T) {
	f := AnalyticsExecutorFactory{}

	start := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 3, 25, 0, 0, 0, 0, time.UTC)

	q := squirrel.Select("*").From("t")
	q = f.BuildPushdownFilter(q, "orgid", start, end, "accounts")

	sql, args, err := q.ToSql()

	assert.NilError(t, err)
	assert.Equal(t, `SELECT * FROM t WHERE "main"."trigger_object_type" = ? AND "main"."year" = ? and "main"."month" between ? and ?`, sql)
	assert.DeepEqual(t, []any{"accounts", 2024, time.March, time.March}, args)
}
