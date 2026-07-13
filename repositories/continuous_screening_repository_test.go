package repositories

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestContinuousScreeningClientDataIndexingAggregateQueryUsesIndexVersion(t *testing.T) {
	orgId := uuid.New()
	indexVersion := "20260713123000-001"

	sql, args, err := continuousScreeningClientDataIndexingAggregateQuery(
		orgId, &indexVersion).ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "df.created_at AS job_date")
	require.NotContains(t, sql, "date_trunc")
	require.Contains(t, sql, "df.version <= $3")
	require.Equal(t, []any{
		models.ContinuousScreeningDatasetFileTypeFull.String(),
		orgId.String(),
		indexVersion,
	}, args)
}

func TestContinuousScreeningClientDataIndexingAggregateQueryWithoutIndexVersionReturnsDatabaseHistory(t *testing.T) {
	orgId := uuid.New()

	sql, args, err := continuousScreeningClientDataIndexingAggregateQuery(orgId, nil).ToSql()

	require.NoError(t, err)
	require.NotContains(t, sql, "FALSE")
	require.NotContains(t, sql, "df.version <=")
	require.Equal(t, []any{
		models.ContinuousScreeningDatasetFileTypeFull.String(),
		orgId.String(),
	}, args)
}

func TestContinuousScreeningClientDataIndexingPendingQueryUsesIndexVersion(t *testing.T) {
	orgId := uuid.New()
	indexVersion := "20260713123000-001"

	sql, args, err := continuousScreeningClientDataIndexingPendingQuery(
		orgId, &indexVersion).ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "(df.id IS NULL OR df.version > $3)")
	require.Equal(t, []any{
		models.ContinuousScreeningDatasetFileTypeFull.String(),
		orgId.String(),
		indexVersion,
	}, args)
}

func TestContinuousScreeningClientDataIndexingPendingQueryWithoutIndexVersionCountsUnassignedRows(t *testing.T) {
	orgId := uuid.New()

	sql, args, err := continuousScreeningClientDataIndexingPendingQuery(orgId, nil).ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "df.id IS NULL")
	require.NotContains(t, sql, "df.version >")
	require.Equal(t, []any{
		models.ContinuousScreeningDatasetFileTypeFull.String(),
		orgId.String(),
	}, args)
}
