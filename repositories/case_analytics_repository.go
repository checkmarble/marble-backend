package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/jackc/pgx/v5"
)

func (repo MarbleDbRepository) CasesCreatedByTimeStats(
	ctx context.Context,
	exec Executor,
	filters analytics.CaseAnalyticsFilter,
) ([]analytics.CasesCreated, error) {
	query := NewQueryBuilder().
		Select(
			fmt.Sprintf("(created_at + interval '%d s')::date as date", filters.TzOffsetSeconds),
			"count(*) as count",
		).
		From(dbmodels.TABLE_CASES).
		Where(squirrel.Eq{
			"inbox_id": filters.InboxIds,
			"org_id":   filters.OrgId,
		}).
		Where(squirrel.And{
			squirrel.GtOrEq{"created_at": filters.Start},
			squirrel.Lt{"created_at": filters.End},
		}).
		GroupBy("date").
		OrderBy("date")

	if filters.AssignedUserId != nil {
		query = query.Where(squirrel.Eq{"assigned_to": *filters.AssignedUserId})
	}

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (analytics.CasesCreated, error) {
		var res analytics.CasesCreated
		err := row.Scan(&res.Date, &res.Count)
		return res, err
	})
}

func (repo MarbleDbRepository) CasesFalsePositiveRateByTimeStats(
	ctx context.Context,
	exec Executor,
	filters analytics.CaseAnalyticsFilter,
) ([]analytics.CasesFalsePositiveRate, error) {
	query := NewQueryBuilder().
		Select(
			fmt.Sprintf("(created_at + interval '%d s')::date as date", filters.TzOffsetSeconds),
			"count(*) as total_closed",
			"count(*) filter (where outcome = 'false_positive') as false_positives",
		).
		From(dbmodels.TABLE_CASES).
		Where(squirrel.Eq{
			"inbox_id": filters.InboxIds,
			"org_id":   filters.OrgId,
			"status":   "closed",
		}).
		Where(squirrel.And{
			squirrel.GtOrEq{"created_at": filters.Start},
			squirrel.Lt{"created_at": filters.End},
		}).
		GroupBy("date").
		OrderBy("date")

	if filters.AssignedUserId != nil {
		query = query.Where(squirrel.Eq{"assigned_to": *filters.AssignedUserId})
	}

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (
		analytics.CasesFalsePositiveRate, error,
	) {
		var res analytics.CasesFalsePositiveRate
		err := row.Scan(&res.Date, &res.TotalClosed, &res.FalsePositives)
		return res, err
	})
}
