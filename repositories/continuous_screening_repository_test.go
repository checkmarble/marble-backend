package repositories

import (
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestContinuousScreeningDatasetUpdatesQueryUsesSharedJobScope(t *testing.T) {
	orgId := uuid.New()
	pagination := models.PaginationAndSorting{
		Sorting: models.SortingFieldCreatedAt,
		Order:   models.SortingOrderDesc,
		Limit:   25,
	}

	query, err := continuousScreeningDatasetUpdatesQuery(
		orgId, models.ScreeningProviderLexisNexis, pagination)
	require.NoError(t, err)

	sql, args, err := query.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "LEFT JOIN LATERAL")
	require.Contains(t, sql, "FROM continuous_screening_update_jobs AS ucs")
	require.Contains(t, sql, "LEFT JOIN continuous_screening_job_offsets AS off")
	require.Contains(t, sql, "ucs.org_id = $1")
	require.Contains(t, sql, "ucs.provider = $2")
	require.Contains(t, sql, "ucs.continuous_screening_dataset_update_id = ds.id")
	require.Contains(t, sql, "ORDER BY ds.created_at DESC, ds.id DESC LIMIT 25")
	require.Equal(t, []any{orgId.String(), models.ScreeningProviderLexisNexis}, args)

	offset := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	pagination.OffsetId = "018f6f98-b70d-7e16-a210-cc7b5089ee11"
	query = applyContinuousScreeningKeysetCondition(query, "ds", pagination, offset)

	sql, args, err = query.ToSql()
	require.NoError(t, err)
	require.Contains(t, sql, "(ds.created_at, ds.id) < ($3, $4)")
	require.Equal(t, []any{
		orgId.String(),
		models.ScreeningProviderLexisNexis,
		offset,
		pagination.OffsetId,
	}, args)
}

func TestContinuousScreeningUpdateJobDetailsQueryUsesSharedJobScope(t *testing.T) {
	orgId := uuid.New()
	pagination := models.PaginationAndSorting{
		Sorting: models.SortingFieldUpdatedAt,
		Order:   models.SortingOrderAsc,
		Limit:   10,
	}

	query, err := continuousScreeningUpdateJobDetailsQuery(
		orgId, models.ScreeningProviderOpenSanctions, pagination)
	require.NoError(t, err)

	sql, args, err := query.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "FROM continuous_screening_update_jobs AS ucs")
	require.Contains(t, sql, "LEFT JOIN continuous_screening_job_offsets AS off")
	require.Contains(t, sql, "LEFT JOIN continuous_screening_configs AS cs")
	require.Contains(t, sql, "LEFT JOIN continuous_screening_dataset_updates AS ds")
	require.Contains(t, sql, "LEFT JOIN continuous_screening_job_errors AS err")
	require.Contains(t, sql, "ucs.org_id = $1")
	require.Contains(t, sql, "ucs.provider = $2")
	require.NotContains(t, sql, "JOIN organizations")
	require.Contains(t, sql, "ORDER BY ucs.updated_at ASC, ucs.id ASC LIMIT 10")
	require.Equal(t, []any{orgId.String(), models.ScreeningProviderOpenSanctions}, args)

	// The job timings come from dedicated columns: created_at is the enqueue time and updated_at
	// moves on every status write, so neither describes when the job ran.
	require.Contains(t, sql, "ucs.started_at AS job_start")
	require.Contains(t, sql, "ucs.finished_at AS job_end")
	require.NotContains(t, sql, "ucs.created_at AS job_start")
	require.NotContains(t, sql, "ucs.updated_at AS job_end")
	require.Contains(t, sql, "GROUP BY ucs.id, ucs.status, ucs.created_at, ucs.updated_at, "+
		"ucs.started_at, ucs.finished_at")
}

func TestContinuousScreeningListQueriesValidateSorting(t *testing.T) {
	pagination := models.PaginationAndSorting{
		Sorting: models.SortingFieldUpdatedAt,
		Order:   models.SortingOrderDesc,
		Limit:   10,
	}

	_, err := continuousScreeningDatasetUpdatesQuery(
		uuid.New(), models.ScreeningProviderOpenSanctions, pagination)
	require.ErrorIs(t, err, models.BadParameterError)

	_, err = continuousScreeningUpdateJobDetailsQuery(
		uuid.New(), models.ScreeningProviderOpenSanctions, pagination)
	require.NoError(t, err)
}

func TestApplyContinuousScreeningKeysetConditionUsesOrderDirection(t *testing.T) {
	offset := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)

	for _, testCase := range []struct {
		name     string
		order    models.SortingOrder
		operator string
	}{
		{name: "descending", order: models.SortingOrderDesc, operator: "<"},
		{name: "ascending", order: models.SortingOrderAsc, operator: ">"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			query := applyContinuousScreeningKeysetCondition(
				NewQueryBuilder().Select("ucs.id").From("jobs ucs"),
				"ucs",
				models.PaginationAndSorting{
					OffsetId: "018f6f98-b70d-7e16-a210-cc7b5089ee11",
					Sorting:  models.SortingFieldUpdatedAt,
					Order:    testCase.order,
				},
				offset,
			)

			sql, args, err := query.ToSql()

			require.NoError(t, err)
			require.Contains(t, sql,
				"(ucs.updated_at, ucs.id) "+testCase.operator+" ($1, $2)")
			require.Equal(t, []any{
				offset,
				"018f6f98-b70d-7e16-a210-cc7b5089ee11",
			}, args)
		})
	}
}

func TestContinuousScreeningClientDataIndexingAggregateQueryUsesIndexVersion(t *testing.T) {
	orgId := uuid.New()
	indexVersion := "20260713123000-001"

	sql, args, err := continuousScreeningClientDataIndexingAggregateQuery(
		orgId, &indexVersion).ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "df.created_at AS job_date")
	require.NotContains(t, sql, "date_trunc")
	require.NotContains(t, sql, "df.provider")
	require.Contains(t, sql, "df.version <= $3")
	require.Equal(t, []any{
		orgId.String(),
		models.ContinuousScreeningDatasetFileTypeFull.String(),
		indexVersion,
	}, args)
}

// The dataset file is the paginated entity: it anchors the FROM, df.id is the row identity, and
// delta tracks are only counted — including for a file that has none.
func TestContinuousScreeningClientDataIndexingAggregateQueryIsAnchoredOnDatasetFiles(t *testing.T) {
	orgId := uuid.New()

	sql, _, err := continuousScreeningClientDataIndexingAggregateQuery(orgId, nil).ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "FROM continuous_screening_dataset_files AS df")
	require.Contains(t, sql, "LEFT JOIN continuous_screening_delta_tracks AS dt "+
		"ON dt.dataset_file_id = df.id AND dt.org_id = df.org_id")
	require.Contains(t, sql, "COUNT(dt.id) AS total_items")
	require.Contains(t, sql, "GROUP BY df.id")
	require.NotContains(t, sql, "MIN(")
	require.NotContains(t, sql, "object_type")
}

func TestContinuousScreeningClientDataIndexingAggregateQueryWithoutIndexVersionReturnsDatabaseHistory(t *testing.T) {
	orgId := uuid.New()

	sql, args, err := continuousScreeningClientDataIndexingAggregateQuery(
		orgId, nil).ToSql()

	require.NoError(t, err)
	require.NotContains(t, sql, "FALSE")
	require.NotContains(t, sql, "df.version <=")
	require.NotContains(t, sql, "df.provider")
	require.Equal(t, []any{
		orgId.String(),
		models.ContinuousScreeningDatasetFileTypeFull.String(),
	}, args)
}

// The cursor lookup shares this scope with the listing, so an id outside the org, of another file
// type, or above the indexed version cannot be used as a cursor.
func TestContinuousScreeningClientDataFilesQueryScopesTheCursorLookup(t *testing.T) {
	orgId := uuid.New()
	indexVersion := "20260713123000-001"
	offsetId := "018f6f98-b70d-7e16-a210-cc7b5089ee11"

	sql, args, err := continuousScreeningClientDataFilesQuery(orgId, &indexVersion,
		"df.created_at AS offset_value").
		Where(squirrel.Eq{"df.id": offsetId}).
		ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "SELECT df.created_at AS offset_value")
	require.Contains(t, sql, "FROM continuous_screening_dataset_files AS df")
	require.Contains(t, sql, "df.org_id = $1")
	require.Contains(t, sql, "df.file_type = $2")
	require.Contains(t, sql, "df.version <= $3")
	require.Contains(t, sql, "df.id = $4")
	require.Equal(t, []any{
		orgId.String(),
		models.ContinuousScreeningDatasetFileTypeFull.String(),
		indexVersion,
		offsetId,
	}, args)
}

func TestContinuousScreeningClientDataIndexingKeysetConditionUsesDatasetFileCursor(t *testing.T) {
	orgId := uuid.New()
	offset := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	pagination := models.PaginationAndSorting{
		OffsetId: "018f6f98-b70d-7e16-a210-cc7b5089ee11",
		Sorting:  models.SortingFieldCreatedAt,
		Order:    models.SortingOrderDesc,
		Limit:    25,
	}

	query := applyContinuousScreeningKeysetCondition(
		continuousScreeningClientDataIndexingAggregateQuery(orgId, nil),
		"df", pagination, offset)

	sql, args, err := query.ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "(df.created_at, df.id) < ($3, $4)")
	require.Equal(t, []any{
		orgId.String(),
		models.ContinuousScreeningDatasetFileTypeFull.String(),
		offset,
		pagination.OffsetId,
	}, args)
}

func TestContinuousScreeningClientDataIndexingPendingQueryUsesIndexVersion(t *testing.T) {
	orgId := uuid.New()
	indexVersion := "20260713123000-001"

	sql, args, err := continuousScreeningClientDataIndexingPendingQuery(
		orgId, &indexVersion).ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "SELECT COUNT(*) FROM (")
	require.Contains(t, sql, "LIMIT 1000")
	require.NotContains(t, sql, "df.provider")
	require.Contains(t, sql, "(df.id IS NULL OR df.version > $3)")
	require.Equal(t, []any{
		models.ContinuousScreeningDatasetFileTypeFull.String(),
		orgId.String(),
		indexVersion,
	}, args)
}

func TestContinuousScreeningClientDataIndexingPendingQueryWithoutIndexVersionCountsUnassignedRows(t *testing.T) {
	orgId := uuid.New()

	sql, args, err := continuousScreeningClientDataIndexingPendingQuery(
		orgId, nil).ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "SELECT COUNT(*) FROM (")
	require.Contains(t, sql, "LIMIT 1000")
	require.Contains(t, sql, "df.id IS NULL")
	require.NotContains(t, sql, "df.version >")
	require.NotContains(t, sql, "df.provider")
	require.Equal(t, []any{
		models.ContinuousScreeningDatasetFileTypeFull.String(),
		orgId.String(),
	}, args)
}
