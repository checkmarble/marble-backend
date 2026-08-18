package repositories

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
)

// The builder writes DDL and an INSERT..SELECT that no query builder checks for it, so what it
// emits is asserted directly.

// graphBuilderExecutor adapts a pgxmock pool to repositories.Executor and repositories.Transaction
// (SwapGraphTable takes a Transaction). Its query matcher always accepts and instead records every
// statement handed to Exec: several of these tests assert on generated DDL - nonce-suffixed names,
// a diff between two runs - that isn't known ahead of time to declare as a pgxmock expectation.
type graphBuilderExecutor struct {
	pool       pgxmock.PgxPoolIface
	schemaType models.DatabaseSchemaType
	statements []string
}

func newGraphBuilderExecutor(t *testing.T) *graphBuilderExecutor {
	t.Helper()

	exec := &graphBuilderExecutor{schemaType: models.DATABASE_SCHEMA_TYPE_CLIENT}

	pool, err := pgxmock.NewPool(pgxmock.QueryMatcherOption(pgxmock.QueryMatcherFunc(
		func(_, actualSQL string) error {
			exec.statements = append(exec.statements, actualSQL)
			return nil
		},
	)))
	require.NoError(t, err)
	exec.pool = pool

	return exec
}

// expectStatements queues n Exec expectations, so pgxmock doesn't reject them as unexpected
// calls. The SQL text itself is asserted afterwards from exec.statements, not here.
func (e *graphBuilderExecutor) expectStatements(n int) {
	for range n {
		e.pool.ExpectExec(".*").WillReturnResult(pgxmock.NewResult("", 0))
	}
}

// expectStatementsWithArgs is expectStatements for the statements that take bind parameters, which
// pgxmock rejects as unexpected unless the argument list is declared.
func (e *graphBuilderExecutor) expectStatementsWithArgs(n int, args ...any) {
	for range n {
		e.pool.ExpectExec(".*").WithArgs(args...).WillReturnResult(pgxmock.NewResult("", 0))
	}
}

// joined returns every statement recorded, so a test can assert on the sequence as a whole.
func (e *graphBuilderExecutor) joined() string {
	return strings.Join(e.statements, "\n;\n")
}

func (e *graphBuilderExecutor) DatabaseSchema() models.DatabaseSchema {
	return models.DatabaseSchema{SchemaType: e.schemaType, Schema: "org-test"}
}

func (e *graphBuilderExecutor) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return e.pool.Exec(ctx, sql, args...)
}

func (e *graphBuilderExecutor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return e.pool.Query(ctx, sql, args...)
}

func (e *graphBuilderExecutor) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return e.pool.QueryRow(ctx, sql, args...)
}

func (e *graphBuilderExecutor) Begin(_ context.Context) (Transaction, error) { return e, nil }
func (e *graphBuilderExecutor) Cache(_ context.Context) *RedisExecutor       { return nil }
func (e *graphBuilderExecutor) RawTx() pgx.Tx                                { return e.pool }
func (e *graphBuilderExecutor) Commit(_ context.Context) error               { return nil }
func (e *graphBuilderExecutor) Rollback(_ context.Context) error             { return nil }

func TestGraphBuilder_RefusesAMarbleExecutor(t *testing.T) {
	// The adjacency table lives in the organization's own schema. Pointing any of this at the
	// marble database would create tables in it.
	exec := newGraphBuilderExecutor(t)
	exec.schemaType = models.DATABASE_SCHEMA_TYPE_MARBLE

	repo := MarbleDbRepository{}

	assert.Error(t, repo.CreateGraphBuildTable(context.Background(), exec))
	assert.Error(t, repo.IndexGraphBuildTable(context.Background(), exec))
	assert.Error(t, repo.AnalyzeGraphBuildTable(context.Background(), exec))
	assert.Error(t, repo.DropGraphBuildTable(context.Background(), exec))

	assert.Error(t, repo.SwapGraphTable(context.Background(), exec))

	fields := []models.Field{{Name: "iban", DataType: models.String}}

	_, err := repo.PopulateGraphBuildTable(context.Background(), exec, "accounts", fields)
	assert.Error(t, err)

	_, err = repo.ReplayGraphRows(context.Background(), exec, time.Now())
	assert.Error(t, err)

	_, err = repo.ReconcileGraphFromOld(context.Background(), exec, time.Now())
	assert.Error(t, err)

	_, err = repo.GraphReplayWatermark(context.Background(), exec)
	assert.Error(t, err)

	_, err = repo.UpsertGraphRows(context.Background(), exec, "accounts", fields, []string{"a"})
	assert.Error(t, err)

	_, err = repo.RetractGraphRows(context.Background(), exec, "accounts", fields, []string{"a"})
	assert.Error(t, err)

	assert.Empty(t, exec.statements, "nothing is executed against the wrong database")
}

func TestGraphBuilder_CreateBuildTableIsUnloggedAndUnindexed(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	exec.expectStatements(3) // drop build leftover, drop aside leftover, then create

	require.NoError(t, MarbleDbRepository{}.CreateGraphBuildTable(context.Background(), exec))

	sql := exec.joined()
	assert.Contains(t, sql, `drop table if exists "org-test"."_graph_build"`,
		"a table left behind by a failed run is cleared first")
	assert.Contains(t, sql, `drop table if exists "org-test"."_graph_old"`,
		"a run that died between the swap and the reconcile orphans the previous generation")
	assert.Contains(t, sql, `create unlogged table "org-test"."_graph_build"`)
	assert.NotContains(t, sql, "primary key", "indexes cost less once the rows are in")
	assert.NotContains(t, sql, "create index")
}

func TestGraphBuilder_PopulateUnpivotsEveryFieldInOneScan(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	exec.expectStatements(1)

	_, err := MarbleDbRepository{}.PopulateGraphBuildTable(context.Background(),
		exec, "accounts", []models.Field{
			{Name: "iban", DataType: models.String},
			{Name: "object_id", DataType: models.String},
			{Name: "user_id", DataType: models.String},
		})
	require.NoError(t, err)

	require.Len(t, exec.statements, 1, "the source table is read exactly once")
	sql := exec.statements[0]

	assert.Contains(t, sql, `insert into "org-test"."_graph_build"`)
	assert.Contains(t, sql, `from "org-test"."accounts" t`)
	assert.Contains(t, sql, `select 'accounts', t.object_id`,
		"a record is identified by its object_id, which is what links resolve against")

	// One VALUES entry per field, each cast to a common representation so values can be matched
	// across tables.
	assert.Contains(t, sql, `('iban', t."iban"::text)`)
	assert.Contains(t, sql, `('object_id', t."object_id"::text)`)
	assert.Contains(t, sql, `('user_id', t."user_id"::text)`)

	assert.Contains(t, sql, `t.valid_until = 'infinity'`, "superseded versions are not in the graph")
	assert.Contains(t, sql, "v.field_value is not null")
	assert.Contains(t, sql, "v.field_value <> ''",
		"the walk treats an empty value as absent, so it must not be stored")
}

func TestGraphBuilder_ProjectionIsCanonicalPerDataType(t *testing.T) {
	// This projection is the equality function of the whole graph: two records are related when
	// their projections are byte-equal. Each type therefore needs the representation that makes
	// "equal" mean what someone declaring a relation over the field would expect.
	tests := []struct {
		name     string
		field    models.Field
		expected string
		why      string
	}{
		{
			name:     "a string is taken as it is",
			field:    models.Field{Name: "iban", DataType: models.String},
			expected: `t."iban"::text`,
		},
		{
			name:     "an ip address keeps its netmask",
			field:    models.Field{Name: "ip", DataType: models.IpAddress},
			expected: `t."ip"::text`,
			why: "the mask is part of the value: dropping it would render 1.2.3.0/24 and " +
				"1.2.3.0/25 identically and join records from different subnets",
		},
		{
			name:     "coordinates are rendered as readable EWKT",
			field:    models.Field{Name: "loc", DataType: models.Coords},
			expected: `st_asewkt(t."loc")`,
			why:      "a cast yields hex EWKB, and a connector node is named after the value it stands for",
		},
		{
			name:     "a timestamp is normalised to UTC",
			field:    models.Field{Name: "at", DataType: models.Timestamp},
			expected: `(t."at" at time zone 'utc')::text`,
			why:      "a cast renders in the session TimeZone, so the value would depend on where the worker runs",
		},
		{
			name:     "an int falls back to a plain cast",
			field:    models.Field{Name: "amount", DataType: models.Int},
			expected: `t."amount"::text`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, graphFieldProjection(tt.field), tt.why)
		})
	}
}

func TestGraphBuilder_PopulateSkipsATypeWithNoFields(t *testing.T) {
	exec := newGraphBuilderExecutor(t)

	rows, err := MarbleDbRepository{}.PopulateGraphBuildTable(context.Background(),
		exec, "accounts", nil)

	require.NoError(t, err)
	assert.Zero(t, rows)
	assert.Empty(t, exec.statements, "an empty VALUES list is not valid SQL")
}

func TestGraphBuilder_IndexesMatchTheWalksQueryShapes(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	exec.expectStatements(4) // primary key, lookup index, updated_at index, statistics

	require.NoError(t, MarbleDbRepository{}.IndexGraphBuildTable(context.Background(), exec))

	sql := exec.joined()
	// Hydrating a record's fields, and finding the records carrying a value.
	assert.Contains(t, sql, "primary key (record_type, record_id, field_name)")
	assert.Contains(t, sql, "(record_type, field_name, field_value)")

	// Not a walk shape: this one serves the replay that carries incremental rows forward.
	assert.Contains(t, sql, "using brin (updated_at)",
		"a btree here would tax every incremental upsert to serve one range scan per build")

	// The three lookup columns are correlated, so estimating them independently lands orders of
	// magnitude low — which would make every hypernode count the walk reports meaningless.
	assert.Contains(t, sql, "create statistics")
	assert.Contains(t, sql, "(ndistinct, dependencies, mcv) on record_type, field_name, field_value")

	// Analyzing is a separate step so the replay can run between the two — it inserts rows, and
	// statistics gathered before them would not describe the table that gets swapped in.
	assert.NotContains(t, sql, "analyze ")
}

func TestGraphBuilder_AnalyzeTargetsTheBuildTable(t *testing.T) {
	// A freshly built table has no statistics of its own at all until this runs, which would leave
	// the walk's hypernode estimate at the planner's no-stats default.
	exec := newGraphBuilderExecutor(t)
	exec.expectStatements(1)

	require.NoError(t, MarbleDbRepository{}.AnalyzeGraphBuildTable(context.Background(), exec))

	assert.Equal(t, []string{`analyze "org-test"."_graph_build"`}, exec.statements)
}

// TestGraphBuilder_SwapOnlyRenames is the one that protects walk availability. Graph requests time out
// at five seconds and the middleware cancels the request context, so a walk stuck behind this lock does
// not return slowly — it returns HTTP 408. Anything but the renames belongs outside it.
func TestGraphBuilder_SwapOnlyRenames(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	exec.expectStatements(1) // lock_timeout
	exec.pool.ExpectQuery(".*").WithArgs(graphTable, "org-test").WillReturnRows(
		pgxmock.NewRows([]string{"exists"}).AddRow(true))
	exec.expectStatements(2) // the two renames

	require.NoError(t, MarbleDbRepository{}.SwapGraphTable(context.Background(), exec))

	sql := exec.joined()
	assert.Contains(t, sql, "set local lock_timeout",
		"a walk in flight must delay the swap, not block every later reader behind it")

	// The previous generation is moved aside rather than dropped, which is what lets the catch-up run
	// after the swap instead of under this lock.
	assert.Contains(t, sql, `alter table "org-test"."_graph" rename to "_graph_old"`)
	assert.Contains(t, sql, `alter table "org-test"."_graph_build" rename to "_graph"`)

	assert.NotContains(t, sql, "lock table",
		"the renames take their own lock; asking for it explicitly would only widen the window")
	assert.NotContains(t, sql, "insert into", "no catch-up may run under this lock")
	assert.NotContains(t, sql, "drop table", "dropping here would unlink files while readers wait")
	assert.NotContains(t, sql, "statement_timeout")

	// Index/constraint/statistics names are nonce-suffixed at creation time (see
	// TestGraphBuilder_IndexNamesDoNotCollideAcrossBuilds), so nothing needs renaming to free up a
	// fixed name for the next build.
	assert.NotContains(t, sql, "alter index")
	assert.NotContains(t, sql, "alter statistics")

	// Aside first: the second rename would otherwise collide with the live name.
	assert.Less(t, strings.Index(sql, `rename to "_graph_old"`), strings.Index(sql, `rename to "_graph"`))
}

func TestGraphBuilder_SwapHasNothingToMoveAsideOnAFirstBuild(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	exec.expectStatements(1) // lock_timeout
	exec.pool.ExpectQuery(".*").WithArgs(graphTable, "org-test").WillReturnRows(
		pgxmock.NewRows([]string{"exists"}).AddRow(false))
	exec.expectStatements(1) // the one rename

	require.NoError(t, MarbleDbRepository{}.SwapGraphTable(context.Background(), exec))

	sql := exec.joined()
	assert.NotContains(t, sql, "_graph_old", "renaming a table that does not exist would fail the build")
	assert.Contains(t, sql, `alter table "org-test"."_graph_build" rename to "_graph"`)

	require.NoError(t, exec.pool.ExpectationsWereMet())
}

func TestGraphBuilder_ReconcileLocksOnlyTheDiscardedTable(t *testing.T) {
	// The lock is what makes one pass complete — a rename does not invalidate an OID, so writers and
	// readers that resolved the old table are still on it after the swap and this waits for them. It
	// costs nothing, because no walk reads `_graph_old` by name.
	exec := newGraphBuilderExecutor(t)
	exec.pool.ExpectQuery(".*").WithArgs(graphOldTable, "org-test").WillReturnRows(
		pgxmock.NewRows([]string{"exists"}).AddRow(true))
	exec.expectStatements(1)                           // lock table
	exec.expectStatementsWithArgs(1, pgxmock.AnyArg()) // replay
	exec.expectStatements(1)                           // drop

	_, err := MarbleDbRepository{}.ReconcileGraphFromOld(context.Background(), exec, time.Now())
	require.NoError(t, err)

	sql := exec.joined()
	assert.Contains(t, sql, `lock table "org-test"."_graph_old" in access exclusive mode`)
	assert.NotContains(t, sql, `lock table "org-test"."_graph" in`,
		"locking the live table is the thing this design exists to avoid")

	// Carrying the rows the other way: out of the previous generation, into the new live table.
	assert.Contains(t, sql, `insert into "org-test"."_graph" as g`)
	assert.Contains(t, sql, `from "org-test"."_graph_old"`)
	assert.Contains(t, sql, "where updated_at >= $1")

	lock := strings.Index(sql, "lock table")
	replay := strings.Index(sql, "insert into")
	drop := strings.Index(sql, "drop table")

	require.NotEqual(t, -1, drop)
	assert.Less(t, lock, replay, "a straggler committing after the replay read would be lost")
	assert.Less(t, replay, drop, "the rows have to be carried over before their only copy goes away")
}

func TestGraphBuilder_ReconcileIsANoOpWithoutAPreviousGeneration(t *testing.T) {
	// A first build renamed nothing aside, and a build whose reconcile already ran has nothing left.
	exec := newGraphBuilderExecutor(t)
	exec.pool.ExpectQuery(".*").WithArgs(graphOldTable, "org-test").WillReturnRows(
		pgxmock.NewRows([]string{"exists"}).AddRow(false))

	replayed, err := MarbleDbRepository{}.ReconcileGraphFromOld(context.Background(), exec, time.Now())

	require.NoError(t, err)
	assert.Zero(t, replayed)
	assert.NotContains(t, exec.joined(), "lock table")
	assert.NotContains(t, exec.joined(), "drop table")
}

func TestGraphBuilder_IndexNamesDoNotCollideAcrossBuilds(t *testing.T) {
	// Each build must get its own index/constraint/statistics names: since the swap no longer
	// renames them, a repeated fixed name would collide with the previous cycle's objects still
	// attached to the live table.
	first := newGraphBuilderExecutor(t)
	first.expectStatements(4)
	second := newGraphBuilderExecutor(t)
	second.expectStatements(4)

	require.NoError(t, MarbleDbRepository{}.IndexGraphBuildTable(context.Background(), first))
	require.NoError(t, MarbleDbRepository{}.IndexGraphBuildTable(context.Background(), second))

	assert.NotEqual(t, first.joined(), second.joined(), "two builds must not produce identical DDL")
}

func TestPgStringLiteral(t *testing.T) {
	assert.Equal(t, `'accounts'`, pgClientDataIdentifierString("accounts"))
	assert.Equal(t, `'it''s'`, pgClientDataIdentifierString("it's"))
}
