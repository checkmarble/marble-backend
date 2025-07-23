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
	q = f.BuildPushdownFilter(q, "orgid", start, end)

	sql, args, err := q.ToSql()

	assert.NilError(t, err)
	assert.Equal(t, "SELECT * FROM t WHERE year in ? AND ((year = ? AND month between ? and 12) OR (year = ? AND month between 1 and ?))", sql)
	assert.DeepEqual(t, []any{[]int{2025, 2026}, 2024, time.March, 2027, time.October}, args)
}

func TestSameYears(t *testing.T) {
	f := AnalyticsExecutorFactory{}

	start := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 10, 3, 0, 0, 0, 0, time.UTC)

	q := squirrel.Select("*").From("t")
	q = f.BuildPushdownFilter(q, "orgid", start, end)

	sql, args, err := q.ToSql()

	assert.NilError(t, err)
	assert.Equal(t, "SELECT * FROM t WHERE year = ? and month between ? and ?", sql)
	assert.DeepEqual(t, []any{2024, time.March, time.October}, args)
}

func TestSameMonths(t *testing.T) {
	f := AnalyticsExecutorFactory{}

	start := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 3, 25, 0, 0, 0, 0, time.UTC)

	q := squirrel.Select("*").From("t")
	q = f.BuildPushdownFilter(q, "orgid", start, end)

	sql, args, err := q.ToSql()

	assert.NilError(t, err)
	assert.Equal(t, "SELECT * FROM t WHERE year = ? and month between ? and ?", sql)
	assert.DeepEqual(t, []any{2024, time.March, time.March}, args)
}
