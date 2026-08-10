package repositories

import (
	"context"
	"strings"
	"testing"

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

func (e *graphBuilderExecutor) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (e *graphBuilderExecutor) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row { return nil }
func (e *graphBuilderExecutor) Begin(_ context.Context) (Transaction, error)           { return e, nil }
func (e *graphBuilderExecutor) Cache(_ context.Context) *RedisExecutor                 { return nil }
func (e *graphBuilderExecutor) RawTx() pgx.Tx                                          { return e.pool }
func (e *graphBuilderExecutor) Commit(_ context.Context) error                         { return nil }
func (e *graphBuilderExecutor) Rollback(_ context.Context) error                       { return nil }

func TestGraphBuilder_RefusesAMarbleExecutor(t *testing.T) {
	// The adjacency table lives in the organization's own schema. Pointing any of this at the
	// marble database would create tables in it.
	exec := newGraphBuilderExecutor(t)
	exec.schemaType = models.DATABASE_SCHEMA_TYPE_MARBLE

	repo := GraphBuilderRepositoryPostgresql{}

	assert.Error(t, repo.CreateGraphBuildTable(context.Background(), exec))
	assert.Error(t, repo.IndexGraphBuildTable(context.Background(), exec))
	assert.Error(t, repo.DropGraphBuildTable(context.Background(), exec))
	assert.Error(t, repo.SwapGraphTable(context.Background(), exec))

	_, err := repo.PopulateGraphBuildTable(context.Background(), exec, "accounts",
		[]models.Field{{Name: "iban", DataType: models.String}})
	assert.Error(t, err)

	assert.Empty(t, exec.statements, "nothing is executed against the wrong database")
}

func TestGraphBuilder_CreateBuildTableIsUnloggedAndUnindexed(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	exec.expectStatements(2) // drop-if-exists, then create

	require.NoError(t, GraphBuilderRepositoryPostgresql{}.CreateGraphBuildTable(context.Background(), exec))

	sql := exec.joined()
	assert.Contains(t, sql, `drop table if exists "org-test"."_graph_build"`,
		"a table left behind by a failed run is cleared first")
	assert.Contains(t, sql, `create unlogged table "org-test"."_graph_build"`)
	assert.NotContains(t, sql, "primary key", "indexes cost less once the rows are in")
	assert.NotContains(t, sql, "create index")
}

func TestGraphBuilder_PopulateUnpivotsEveryFieldInOneScan(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	exec.expectStatements(1)

	_, err := GraphBuilderRepositoryPostgresql{}.PopulateGraphBuildTable(context.Background(),
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

	rows, err := GraphBuilderRepositoryPostgresql{}.PopulateGraphBuildTable(context.Background(),
		exec, "accounts", nil)

	require.NoError(t, err)
	assert.Zero(t, rows)
	assert.Empty(t, exec.statements, "an empty VALUES list is not valid SQL")
}

func TestGraphBuilder_IndexesMatchTheWalksTwoQueryShapes(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	exec.expectStatements(4) // primary key, index, statistics, analyze

	require.NoError(t, GraphBuilderRepositoryPostgresql{}.IndexGraphBuildTable(context.Background(), exec))

	sql := exec.joined()
	// Hydrating a record's fields, and finding the records carrying a value.
	assert.Contains(t, sql, "primary key (record_type, record_id, field_name)")
	assert.Contains(t, sql, "(record_type, field_name, field_value)")

	// The three lookup columns are correlated, so estimating them independently lands orders of
	// magnitude low — which would make every hypernode count the walk reports meaningless.
	assert.Contains(t, sql, "create statistics")
	assert.Contains(t, sql, "(ndistinct, dependencies, mcv) on record_type, field_name, field_value")

	// A freshly built table has no statistics of its own at all until this runs.
	assert.Contains(t, sql, `analyze "org-test"."_graph_build"`)
	assert.Less(t, strings.Index(sql, "create statistics"), strings.Index(sql, "analyze "),
		"the statistics object has to exist before the analyze that populates it")
}

func TestGraphBuilder_SwapDoesNotRenameIndexes(t *testing.T) {
	// Index/constraint/statistics names are nonce-suffixed at creation time (see
	// TestGraphBuilder_IndexNamesDoNotCollideAcrossBuilds), so the swap never needs to free up a
	// fixed name for the next build by renaming them.
	exec := newGraphBuilderExecutor(t)
	exec.expectStatements(3) // lock_timeout, drop, rename

	require.NoError(t, GraphBuilderRepositoryPostgresql{}.SwapGraphTable(context.Background(), exec))

	sql := exec.joined()
	assert.Contains(t, sql, "set local lock_timeout",
		"a walk in flight must delay the swap, not block every later reader behind it")
	assert.Contains(t, sql, `drop table if exists "org-test"."_graph"`)
	assert.Contains(t, sql, `alter table "org-test"."_graph_build" rename to "_graph"`)
	assert.NotContains(t, sql, "alter index")
	assert.NotContains(t, sql, "alter statistics")

	// The drop has to precede the rename, or the rename would collide with the live table.
	assert.Less(t, strings.Index(sql, "drop table"), strings.Index(sql, "rename to"))
}

func TestGraphBuilder_IndexNamesDoNotCollideAcrossBuilds(t *testing.T) {
	// Each build must get its own index/constraint/statistics names: since the swap no longer
	// renames them, a repeated fixed name would collide with the previous cycle's objects still
	// attached to the live table.
	first := newGraphBuilderExecutor(t)
	first.expectStatements(4)
	second := newGraphBuilderExecutor(t)
	second.expectStatements(4)

	require.NoError(t, GraphBuilderRepositoryPostgresql{}.IndexGraphBuildTable(context.Background(), first))
	require.NoError(t, GraphBuilderRepositoryPostgresql{}.IndexGraphBuildTable(context.Background(), second))

	assert.NotEqual(t, first.joined(), second.joined(), "two builds must not produce identical DDL")
}

func TestPgStringLiteral(t *testing.T) {
	assert.Equal(t, `'accounts'`, pgClientDataIdentifierString("accounts"))
	assert.Equal(t, `'it''s'`, pgClientDataIdentifierString("it's"))
}
