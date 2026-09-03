package repositories

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
)

// The incremental writer keeps the adjacency table current between builds. Its correctness is
// entirely a property of the SQL it emits — there is no query builder to check it — so what it emits
// is asserted directly, as the builder's is.

var graphIncrementalFields = []models.Field{
	{Name: "iban", DataType: models.String},
	{Name: "object_id", DataType: models.String},
	{Name: "opened_at", DataType: models.Timestamp},
}

func TestGraphIncremental_UpsertRestatesTheLiveVersion(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	exec.expectStatementsWithArgs(1, pgxmock.AnyArg())

	_, err := MarbleDbRepository{}.UpsertGraphRows(context.Background(), exec, "accounts",
		graphIncrementalFields, []string{"acc-1", "acc-2"})
	require.NoError(t, err)

	require.Len(t, exec.statements, 1, "the source table is read once, as in a full build")
	sql := exec.statements[0]

	assert.Contains(t, sql, `insert into "org-test"."_graph" as g`)
	assert.Contains(t, sql, `from "org-test"."accounts" t`)
	assert.Contains(t, sql, `select 'accounts', t.object_id`)

	// Scoped to the records ingested, and to their live version only.
	assert.Contains(t, sql, "t.object_id = any($1)")
	assert.Contains(t, sql, `t.valid_until = 'infinity'`)

	// Same exclusions as the build: the walk cannot tell a stored empty value from an absent one.
	assert.Contains(t, sql, "v.field_value is not null")
	assert.Contains(t, sql, "v.field_value <> ''")
}

func TestGraphIncremental_UpsertConflictTargetIsNotAConstraintName(t *testing.T) {
	// Every build gives the primary key a fresh nonce-suffixed name, so naming the constraint here
	// would work until the first rebuild and then fail against a name that no longer exists.
	exec := newGraphBuilderExecutor(t)
	exec.expectStatementsWithArgs(1, pgxmock.AnyArg())

	_, err := MarbleDbRepository{}.UpsertGraphRows(context.Background(), exec, "accounts",
		graphIncrementalFields, []string{"acc-1"})
	require.NoError(t, err)

	sql := exec.statements[0]
	assert.Contains(t, sql, "on conflict (record_type, record_id, field_name) do update")
	assert.NotContains(t, sql, "on conflict on constraint")
	assert.NotContains(t, sql, graphPkeyName,
		"the primary key's name is nonce-suffixed per build and must not be referenced")
}

func TestGraphIncremental_UpsertDoesNotRewriteAnUnchangedValue(t *testing.T) {
	// idx_graph_lookup covers field_value, so an update that changes nothing still costs an index
	// write — on the ingestion hot path, for every re-ingested record.
	exec := newGraphBuilderExecutor(t)
	exec.expectStatementsWithArgs(1, pgxmock.AnyArg())

	_, err := MarbleDbRepository{}.UpsertGraphRows(context.Background(), exec, "accounts",
		graphIncrementalFields, []string{"acc-1"})
	require.NoError(t, err)

	// Plain inequality suffices because field_value is not null on both sides — see the statement's
	// comment. If that ever changes this needs `is distinct from`, or the guard silently stops firing.
	assert.Contains(t, exec.statements[0], "where g.field_value <> excluded.field_value")
}

func TestGraphIncremental_RetractRemovesWhatTheUpsertCannot(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	exec.expectStatementsWithArgs(1, pgxmock.AnyArg())

	_, err := MarbleDbRepository{}.RetractGraphRows(context.Background(), exec, "accounts",
		graphIncrementalFields, []string{"acc-1"})
	require.NoError(t, err)

	require.Len(t, exec.statements, 1)
	sql := exec.statements[0]

	assert.Contains(t, sql, `delete from "org-test"."_graph" g`)

	// The primary key is (record_type, record_id, field_name), so these two together let the delete
	// find the candidate rows by index rather than scanning the table.
	assert.Contains(t, sql, "g.record_type = 'accounts'")
	assert.Contains(t, sql, "g.record_id = any($1)")

	// A row survives only if the live version still projects a usable value on that field, which is
	// the same condition the upsert inserts under.
	assert.Contains(t, sql, "not exists (")
	assert.Contains(t, sql, "v.field_name = g.field_name")
	assert.Contains(t, sql, `t.valid_until = 'infinity'`)
	assert.Contains(t, sql, "v.field_value is not null")
	assert.Contains(t, sql, "v.field_value <> ''")
}

func TestGraphIncremental_RetractRefusesAnEmptyFieldList(t *testing.T) {
	// With no fields the NOT EXISTS would be vacuously true and the statement would delete every row
	// of these records. GraphIndexedFields never produces an empty list, but the failure mode is
	// silent data loss, so it is refused rather than trusted.
	exec := newGraphBuilderExecutor(t)

	rows, err := MarbleDbRepository{}.RetractGraphRows(context.Background(), exec, "accounts",
		nil, []string{"acc-1"})

	require.NoError(t, err)
	assert.Zero(t, rows)
	assert.Empty(t, exec.statements)
}

func TestGraphIncremental_NothingToDoEmitsNoStatement(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	repo := MarbleDbRepository{}

	_, err := repo.UpsertGraphRows(context.Background(), exec, "accounts", graphIncrementalFields, nil)
	require.NoError(t, err)

	_, err = repo.UpsertGraphRows(context.Background(), exec, "accounts", nil, []string{"acc-1"})
	require.NoError(t, err)

	_, err = repo.RetractGraphRows(context.Background(), exec, "accounts", graphIncrementalFields, nil)
	require.NoError(t, err)

	assert.Empty(t, exec.statements)
}

// TestGraphIncremental_RendersValuesExactlyAsTheBuildDoes is the one that matters most here. Two
// records are related when their stored projections are byte-equal, so if the incremental writer
// rendered a value even slightly differently from the build — a different cast, a different time
// zone — the rows would coexist without ever matching, and the graph would silently lose edges
// rather than fail.
func TestGraphIncremental_RendersValuesExactlyAsTheBuildDoes(t *testing.T) {
	fields := []models.Field{
		{Name: "iban", DataType: models.String},
		{Name: "ip", DataType: models.IpAddress},
		{Name: "loc", DataType: models.Coords},
		{Name: "opened_at", DataType: models.Timestamp},
		{Name: "amount", DataType: models.Int},
	}

	build := newGraphBuilderExecutor(t)
	build.expectStatements(1)
	_, err := MarbleDbRepository{}.PopulateGraphBuildTable(context.Background(), build, "accounts", fields)
	require.NoError(t, err)

	upsert := newGraphBuilderExecutor(t)
	upsert.expectStatementsWithArgs(1, pgxmock.AnyArg())
	_, err = MarbleDbRepository{}.UpsertGraphRows(context.Background(), upsert, "accounts",
		fields, []string{"acc-1"})
	require.NoError(t, err)

	retract := newGraphBuilderExecutor(t)
	retract.expectStatementsWithArgs(1, pgxmock.AnyArg())
	_, err = MarbleDbRepository{}.RetractGraphRows(context.Background(), retract, "accounts",
		fields, []string{"acc-1"})
	require.NoError(t, err)

	reference := graphUnpivotClause(t, build.statements[0])
	assert.NotEmpty(t, reference)
	assert.Equal(t, reference, graphUnpivotClause(t, upsert.statements[0]),
		"a value the incremental writer stores must be byte-identical to the one a build stores")
	assert.Equal(t, reference, graphUnpivotClause(t, retract.statements[0]),
		"the retraction decides what to keep, so it must agree on rendering too")
}

// graphUnpivotClause extracts the `values (...)` list that turns a record's columns into one row per
// field, so two statements can be compared on that fragment alone.
func graphUnpivotClause(t *testing.T, sql string) string {
	t.Helper()

	matches := regexp.MustCompile(`cross join lateral \(values (.*)\) as v\(`).FindStringSubmatch(sql)
	require.Len(t, matches, 2, "statement has no lateral VALUES list: %s", sql)

	return matches[1]
}

func TestGraphIncremental_ReplaySkipsAnOrganizationWithNoLiveTable(t *testing.T) {
	// The first build of an organization has nothing to replay from, and asking anyway would fail the
	// build on a missing relation.
	exec := newGraphBuilderExecutor(t)
	exec.pool.ExpectQuery(".*").WithArgs(graphTable, "org-test").WillReturnRows(
		pgxmock.NewRows([]string{"exists"}).AddRow(false))

	rows, err := MarbleDbRepository{}.ReplayGraphRows(context.Background(), exec, time.Now())

	require.NoError(t, err)
	assert.Zero(t, rows)
	assert.NotContains(t, strings.Join(exec.statements, "\n"), "insert into",
		"the probe runs, the replay does not")
}

func TestGraphIncremental_ReplayCarriesIncrementalRowsForward(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	exec.pool.ExpectQuery(".*").WithArgs(graphTable, "org-test").WillReturnRows(
		pgxmock.NewRows([]string{"exists"}).AddRow(true))
	exec.expectStatementsWithArgs(1, pgxmock.AnyArg())

	since := time.Now()
	_, err := MarbleDbRepository{}.ReplayGraphRows(context.Background(), exec, since)
	require.NoError(t, err)

	sql := exec.joined()

	// From the live table into the one about to be swapped in — the opposite direction to everything
	// else in the build.
	assert.Contains(t, sql, `insert into "org-test"."_graph_build" as g`)
	assert.Contains(t, sql, `from "org-test"."_graph"`)
	assert.Contains(t, sql, "where updated_at >= $1",
		"the watermark is what distinguishes rows the build already has from rows it missed")
	assert.Contains(t, sql, "on conflict (record_type, record_id, field_name) do update")

	require.NoError(t, exec.pool.ExpectationsWereMet())
}
