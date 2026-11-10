package executor_factory

import (
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"
)

func TestBuildUpstreamAttachStatementWithImpersonateRole(t *testing.T) {
	f := AnalyticsExecutorFactory{config: infra.AnalyticsConfig{
		PgConfig: infra.PgConfig{
			ConnectionString: "postgres://iam:secret@localhost/marble?sslmode=require",
			ImpersonateRole:  `marble app\reader`,
		},
	}}

	statement := f.buildUpstreamAttachStatement("pg")

	assert.Equal(t,
		"attach or replace 'postgres://iam:secret@localhost/marble?options=-cenable_seqscan%3D0%20-c%20role%3Dmarble%5C%20app%5C%5Creader&sslmode=require' as pg (type postgres, read_only)",
		statement,
	)
}

func TestDifferentSeparateYears(t *testing.T) {
	f := AnalyticsExecutorFactory{}

	start := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, 10, 3, 0, 0, 0, 0, time.UTC)

	q := squirrel.Select("*").From("t")
	q = f.BuildPushdownFilter(q, uuid.MustParse("12345678-1234-5678-9012-345678901234"), start, end, "accounts")

	sql, args, err := q.ToSql()

	assert.NilError(t, err)
	assert.Equal(t, `SELECT * FROM t WHERE "main"."org_id" = ? AND "main"."trigger_object_type" = ? AND ("main"."year" in ? OR (("main"."year" = ? AND "main"."month" between ? and 12) OR ("main"."year" = ? AND "main"."month" between 1 and ?)))`, sql)
	assert.DeepEqual(t, []any{
		uuid.MustParse("12345678-1234-5678-9012-345678901234"),
		"accounts",
		[]int{2025, 2026},
		2024, time.March, 2027, time.October,
	}, args)
}

func TestSameYears(t *testing.T) {
	f := AnalyticsExecutorFactory{}

	start := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 10, 3, 0, 0, 0, 0, time.UTC)

	q := squirrel.Select("*").From("t")
	q = f.BuildPushdownFilter(q, uuid.MustParse("12345678-1234-5678-9012-345678901234"), start, end, "accounts")

	sql, args, err := q.ToSql()

	assert.NilError(t, err)
	assert.Equal(t, `SELECT * FROM t WHERE "main"."org_id" = ? AND "main"."trigger_object_type" = ? AND ("main"."year" = ? and "main"."month" between ? and ?)`, sql)
	assert.DeepEqual(t, []any{
		uuid.MustParse("12345678-1234-5678-9012-345678901234"),
		"accounts", 2024, time.March, time.October,
	}, args)
}

func TestSameMonths(t *testing.T) {
	f := AnalyticsExecutorFactory{}

	start := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 3, 25, 0, 0, 0, 0, time.UTC)

	q := squirrel.Select("*").From("t")
	q = f.BuildPushdownFilter(q, uuid.MustParse("12345678-1234-5678-9012-345678901234"), start, end, "accounts")

	sql, args, err := q.ToSql()

	assert.NilError(t, err)
	assert.Equal(t, `SELECT * FROM t WHERE "main"."org_id" = ? AND "main"."trigger_object_type" = ? AND ("main"."year" = ? and "main"."month" between ? and ?)`, sql)
	assert.DeepEqual(t, []any{
		uuid.MustParse("12345678-1234-5678-9012-345678901234"),
		"accounts", 2024, time.March, time.March,
	}, args)
}

func TestComputeDuckdbMemoryLimit(t *testing.T) {
	tts := []struct {
		total uint64
		want  string
	}{
		{1 << 30, "512MiB"},
		{2 << 30, "1024MiB"},
		{3_000_000_000, "1430MiB"},
		{4 << 30, "2048MiB"},
		{32 << 30, "2048MiB"},
	}

	for _, tt := range tts {
		assert.Equal(t, tt.want, computeDuckdbMemoryLimit(tt.total))
	}
}

func TestDuckdbMemoryLimitEnvOverride(t *testing.T) {
	t.Setenv("DUCKDB_MEMORY_LIMIT", "1500MB")

	assert.Equal(t, "1500MB", duckdbMemoryLimit())
}
