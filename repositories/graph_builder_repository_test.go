package repositories

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
)

// The builder writes DDL and an INSERT..SELECT that no query builder checks for it, so what it
// emits is asserted directly.

type recordingExecutor struct {
	statements []string
	schemaType models.DatabaseSchemaType
}

func newRecordingExecutor() *recordingExecutor {
	return &recordingExecutor{schemaType: models.DATABASE_SCHEMA_TYPE_CLIENT}
}

func (e *recordingExecutor) DatabaseSchema() models.DatabaseSchema {
	return models.DatabaseSchema{SchemaType: e.schemaType, Schema: "org-test"}
}

func (e *recordingExecutor) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	e.statements = append(e.statements, sql)
	return pgconn.CommandTag{}, nil
}

func (e *recordingExecutor) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (e *recordingExecutor) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row { return nil }
func (e *recordingExecutor) Begin(_ context.Context) (Transaction, error)           { return nil, nil }
func (e *recordingExecutor) Cache(_ context.Context) *RedisExecutor                 { return nil }
func (e *recordingExecutor) RawTx() pgx.Tx                                          { return nil }
func (e *recordingExecutor) Commit(_ context.Context) error                         { return nil }
func (e *recordingExecutor) Rollback(_ context.Context) error                       { return nil }

// joined returns every statement recorded, so a test can assert on the sequence as a whole.
func (e *recordingExecutor) joined() string {
	return strings.Join(e.statements, "\n;\n")
}

func TestGraphBuilder_RefusesAMarbleExecutor(t *testing.T) {
	// The adjacency table lives in the organization's own schema. Pointing any of this at the
	// marble database would create tables in it.
	exec := newRecordingExecutor()
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
	exec := newRecordingExecutor()

	require.NoError(t, GraphBuilderRepositoryPostgresql{}.CreateGraphBuildTable(context.Background(), exec))

	sql := exec.joined()
	assert.Contains(t, sql, `drop table if exists "org-test"."_graph_build"`,
		"a table left behind by a failed run is cleared first")
	assert.Contains(t, sql, `create unlogged table "org-test"."_graph_build"`)
	assert.NotContains(t, sql, "primary key", "indexes cost less once the rows are in")
	assert.NotContains(t, sql, "create index")
}

func TestGraphBuilder_PopulateUnpivotsEveryFieldInOneScan(t *testing.T) {
	exec := newRecordingExecutor()

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
	exec := newRecordingExecutor()

	rows, err := GraphBuilderRepositoryPostgresql{}.PopulateGraphBuildTable(context.Background(),
		exec, "accounts", nil)

	require.NoError(t, err)
	assert.Zero(t, rows)
	assert.Empty(t, exec.statements, "an empty VALUES list is not valid SQL")
}

func TestGraphBuilder_IndexesMatchTheWalksTwoQueryShapes(t *testing.T) {
	exec := newRecordingExecutor()

	require.NoError(t, GraphBuilderRepositoryPostgresql{}.IndexGraphBuildTable(context.Background(), exec))

	sql := exec.joined()
	// Hydrating a record's fields, and finding the records carrying a value.
	assert.Contains(t, sql, "primary key (record_type, record_id, field_name)")
	assert.Contains(t, sql, "(record_type, field_name, field_value)")
}

func TestGraphBuilder_SwapRenamesTheIndexesToo(t *testing.T) {
	// ALTER TABLE ... RENAME TO leaves constraint and index names alone, and index names are
	// schema-scoped: without renaming them the *next* build collides when it recreates them.
	exec := newRecordingExecutor()

	require.NoError(t, GraphBuilderRepositoryPostgresql{}.SwapGraphTable(context.Background(), exec))

	sql := exec.joined()
	assert.Contains(t, sql, "set local lock_timeout",
		"a walk in flight must delay the swap, not block every later reader behind it")
	assert.Contains(t, sql, `drop table if exists "org-test"."_graph"`)
	assert.Contains(t, sql, `alter table "org-test"."_graph_build" rename to "_graph"`)
	assert.Contains(t, sql, `alter index "org-test"."_graph_build_pkey" rename to "_graph_pkey"`)
	assert.Contains(t, sql, `alter index "org-test"."idx__graph_build_lookup" rename to "idx__graph_lookup"`)

	// The drop has to precede the rename, or the rename would collide with the live table.
	assert.Less(t, strings.Index(sql, "drop table"), strings.Index(sql, "rename to"))
}

func TestPgStringLiteral(t *testing.T) {
	assert.Equal(t, `'accounts'`, pgStringLiteral("accounts"))
	assert.Equal(t, `'it''s'`, pgStringLiteral("it's"))
}
