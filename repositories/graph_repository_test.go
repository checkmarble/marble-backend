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

// graphQueryExecutor adapts a pgxmock pool to repositories.Executor for the read side of the
// graph, recording the SQL of every query so a test can assert on what was asked for as well as
// on what came back.
type graphQueryExecutor struct {
	pool       pgxmock.PgxPoolIface
	schemaType models.DatabaseSchemaType
	queries    []string
}

func newGraphQueryExecutor(t *testing.T) *graphQueryExecutor {
	t.Helper()

	exec := &graphQueryExecutor{schemaType: models.DATABASE_SCHEMA_TYPE_CLIENT}

	pool, err := pgxmock.NewPool(pgxmock.QueryMatcherOption(pgxmock.QueryMatcherFunc(
		func(_, actualSQL string) error {
			exec.queries = append(exec.queries, actualSQL)
			return nil
		},
	)))
	require.NoError(t, err)
	exec.pool = pool

	return exec
}

func (e *graphQueryExecutor) DatabaseSchema() models.DatabaseSchema {
	return models.DatabaseSchema{SchemaType: e.schemaType, Schema: "org-test"}
}

func (e *graphQueryExecutor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return e.pool.Query(ctx, sql, args...)
}

func (e *graphQueryExecutor) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (e *graphQueryExecutor) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row { return nil }
func (e *graphQueryExecutor) Begin(_ context.Context) (Transaction, error)           { return nil, nil }
func (e *graphQueryExecutor) Cache(_ context.Context) *RedisExecutor                 { return nil }

func TestGetNodeBatchCaptions_ReadsEveryTypeInOneQuery(t *testing.T) {
	exec := newGraphQueryExecutor(t)
	exec.pool.ExpectQuery(".*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(
		pgxmock.NewRows([]string{"ord", "caption"}).
			AddRow(1, ptr("Jane Doe")).
			AddRow(3, ptr("FR76...")).
			// A record whose caption column is null or empty has nothing to be called by, and is
			// left out rather than reported with an empty caption.
			AddRow(4, (*string)(nil)).
			AddRow(5, ptr("")))

	captions, err := MarbleDbRepository{}.GetNodeBatchCaptions(context.Background(), exec,
		map[string]string{"users": "full_name", "accounts": "iban"},
		[]models.ScoringRecordRef{
			{RecordType: "users", RecordId: "U1"},
			// A connector holds a slot without naming a type, so that the positions the query
			// reports still line up with the caller's nodes.
			{},
			{RecordType: "accounts", RecordId: "A1"},
			{RecordType: "users", RecordId: "U2"},
			{RecordType: "users", RecordId: "U3"},
		})
	require.NoError(t, err)

	assert.Equal(t, []models.GraphResultNodeMetadata{
		{Index: 1, Label: "Jane Doe"},
		{Index: 3, Label: "FR76..."},
	}, captions)

	require.Len(t, exec.queries, 1, "the whole batch costs one round trip")
	sql := exec.queries[0]

	// One branch per record type, since the caption of each lives in a different table under a
	// different column name, unioned into that single statement.
	assert.Contains(t, sql, "unnest($1::text[], $2::text[]) with ordinality")
	assert.Contains(t, sql, "union all")
	assert.Contains(t, sql, `t."iban"::text`)
	assert.Contains(t, sql, `inner join "org-test"."accounts" t on t.object_id = i.id`)
	assert.Contains(t, sql, `where i.type = 'accounts'`)
	assert.Contains(t, sql, `t."full_name"::text`)
	assert.Contains(t, sql, `inner join "org-test"."users" t on t.object_id = i.id`)
	assert.Contains(t, sql, `where i.type = 'users'`)
	assert.Contains(t, sql, "t.valid_until = 'infinity'",
		"only the version of a record the graph was built from has a caption to report")

	// Sorted, so two identical requests are the same statement to plan and cache.
	assert.Less(t, indexOf(t, sql, `'accounts'`), indexOf(t, sql, `'users'`))
}

func TestGetNodeBatchCaptions_SkipsWorkItCannotDo(t *testing.T) {
	exec := newGraphQueryExecutor(t)
	records := []models.ScoringRecordRef{{RecordType: "users", RecordId: "U1"}}

	captions, err := MarbleDbRepository{}.GetNodeBatchCaptions(context.Background(), exec, nil, records)
	require.NoError(t, err)
	assert.Empty(t, captions, "no table declares a caption field, so there is nothing to read")

	captions, err = MarbleDbRepository{}.GetNodeBatchCaptions(context.Background(), exec,
		map[string]string{"users": "full_name"}, nil)
	require.NoError(t, err)
	assert.Empty(t, captions)

	assert.Empty(t, exec.queries)
}

func TestGetNodeBatchCaptions_RefusesAMarbleExecutor(t *testing.T) {
	// Records live in the organization's own schema; the marble database has no such table.
	exec := newGraphQueryExecutor(t)
	exec.schemaType = models.DATABASE_SCHEMA_TYPE_MARBLE

	_, err := MarbleDbRepository{}.GetNodeBatchCaptions(context.Background(), exec,
		map[string]string{"users": "full_name"},
		[]models.ScoringRecordRef{{RecordType: "users", RecordId: "U1"}})
	assert.Error(t, err)
	assert.Empty(t, exec.queries)
}

func ptr[T any](value T) *T {
	return &value
}

func indexOf(t *testing.T, haystack, needle string) int {
	t.Helper()

	idx := strings.Index(haystack, needle)
	require.NotEqual(t, -1, idx, "%q is not in the statement", needle)

	return idx
}
