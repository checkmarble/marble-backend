package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestLifecycleIndexesAndPurge(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	creds, _, _ := setupOrgAndCreds(ctx, t, "lifecycle indexes")
	admin := generateUsecaseWithCredForMarbleAdmin(testUsecases)
	exec, err := admin.NewExecutorFactory().NewClientDbExecutor(ctx, creds.OrganizationId)
	require.NoError(t, err)
	table := pgx.Identifier{exec.DatabaseSchema().Schema, "lifecycle_test"}.Sanitize()
	_, err = exec.Exec(ctx, "CREATE TABLE "+table+" (id bigint PRIMARY KEY, valid_from timestamptz NOT NULL, valid_until timestamptz NOT NULL)")
	require.NoError(t, err)
	_, err = exec.Exec(ctx, "INSERT INTO "+table+` VALUES
		(1, '2020-01-01', 'infinity'),
		(2, '2025-01-01', 'infinity'),
		(3, '2019-01-01', '2020-01-01'),
		(4, '2024-01-01', '2025-01-01')`)
	require.NoError(t, err)
	_, err = exec.Exec(ctx, "INSERT INTO "+table+` (id, valid_from, valid_until)
		SELECT 1000 + n, '2024-01-01'::timestamptz,
		CASE WHEN n % 2 = 0 THEN 'infinity'::timestamptz ELSE '2025-01-01'::timestamptz END
		FROM generate_series(1, 10000) AS n`)
	require.NoError(t, err)
	indexes := []models.ConcreteIndex{
		{TableName: "lifecycle_test", Type: models.IndexTypeLifecycleActive, Indexed: []string{"valid_from"}},
		{TableName: "lifecycle_test", Type: models.IndexTypeLifecycleStale, Indexed: []string{"valid_until"}},
	}
	for i := range indexes {
		indexes[i].Name()
	}
	repo := &admin.Repositories.ClientDbRepository
	require.NoError(t, repo.CreateIndexesConcurrently(ctx, exec, indexes))
	_, err = exec.Exec(ctx, "ANALYZE "+table)
	require.NoError(t, err)
	actual, err := repo.ListAllIndexes(ctx, exec, models.IndexTypeLifecycleActive, models.IndexTypeLifecycleStale)
	require.NoError(t, err)
	for _, index := range indexes {
		require.True(t, models.LifecycleIndexExists(index, actual))
	}

	conn, err := pgPool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "SET enable_seqscan = off")
	require.NoError(t, err)
	defer func() {
		_, err := conn.Exec(ctx, "RESET enable_seqscan")
		require.NoError(t, err)
	}()
	for _, tc := range []struct{ query, index string }{
		{"SELECT id FROM " + table + " WHERE valid_from < '2021-01-01' AND valid_until = 'infinity' ORDER BY valid_from LIMIT 5000", indexes[0].Name()},
		{"SELECT id FROM " + table + " WHERE valid_until < '2021-01-01' ORDER BY valid_until LIMIT 5000", indexes[1].Name()},
	} {
		rows, err := conn.Query(ctx, "EXPLAIN "+tc.query)
		require.NoError(t, err)
		lines, err := pgx.CollectRows(rows, pgx.RowTo[string])
		require.NoError(t, err)
		require.Contains(t, strings.Join(lines, "\n"), tc.index)
	}

	cutoff := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	count, err := admin.Repositories.MarbleDbRepository.DeleteActiveRowsBefore(ctx, exec, "lifecycle_test", cutoff)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	count, err = admin.Repositories.MarbleDbRepository.DeleteStaleRowsBefore(ctx, exec, "lifecycle_test", cutoff)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	rows, err := exec.Query(ctx, "SELECT id FROM "+table+" WHERE id < 1000 ORDER BY id")
	require.NoError(t, err)
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	require.NoError(t, err)
	require.Equal(t, []int64{2, 4}, ids)
	var total int
	require.NoError(t, exec.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&total))
	require.Equal(t, 10002, total)
}
