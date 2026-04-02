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

func (repo MarbleDbRepository) CasesDurationByTimeStats(
	ctx context.Context,
	exec Executor,
	filters analytics.CaseAnalyticsFilter,
) ([]analytics.CasesDuration, error) {
	// Subquery: for each closed case, find the most recent "status_updated" event to "closed".
	// Small gotcha: don't transform '?' to '$' with PlaceholderFormat in NewQueryBuilder in the subquery, as the raw query string is injected below.
	// (the main query handles it)
	subq := squirrel.
		Select(
			"ce.case_id",
			"max(ce.created_at) as closed_at",
		).
		From(dbmodels.TABLE_CASE_EVENTS + " ce").
		Where(squirrel.Eq{
			"ce.event_type": "status_updated",
			"ce.new_value":  "closed",
		}).
		GroupBy("ce.case_id")

	closedAtSQL, closedAtArgs, err := subq.ToSql()
	if err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(
			fmt.Sprintf("(c.created_at + interval '%d s')::date as date", filters.TzOffsetSeconds),
			"sum(extract(epoch from (closed_at - c.created_at)) / 86400) as sum_days",
			"count(*) as count_cases",
			"max(extract(epoch from (closed_at - c.created_at)) / 86400) as max_days",
		).
		From(dbmodels.TABLE_CASES+" c").
		Join(fmt.Sprintf("(%s) ce_agg on ce_agg.case_id = c.id", closedAtSQL), closedAtArgs...).
		Where(squirrel.Eq{
			"c.inbox_id": filters.InboxIds,
			"c.org_id":   filters.OrgId,
			"c.status":   "closed",
		}).
		Where(squirrel.And{
			squirrel.GtOrEq{"c.created_at": filters.Start},
			squirrel.Lt{"c.created_at": filters.End},
		}).
		GroupBy("date").
		OrderBy("date")

	if filters.AssignedUserId != nil {
		query = query.Where(squirrel.Eq{"c.assigned_to": *filters.AssignedUserId})
	}

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (analytics.CasesDuration, error) {
		var res analytics.CasesDuration
		err := row.Scan(&res.Date, &res.SumDays, &res.CountCases, &res.MaxDays)
		return res, err
	})
}

// TODO: Pascal: this is not finished yet, we really want the metric to be displayed by SAR completion date, not case creation. But need to do some work to have that information available.
func (repo MarbleDbRepository) SarCompletedCount(
	ctx context.Context,
	exec Executor,
	filters analytics.CaseAnalyticsFilter,
) (analytics.SarCompletedCount, error) {
	query := NewQueryBuilder().
		Select("count(*) as count").
		From(dbmodels.TABLE_SUSPICIOUS_ACTIVITY_REPORTS + " sar").
		Join(dbmodels.TABLE_CASES + " c on c.id = sar.case_id").
		Where(squirrel.Eq{
			"c.inbox_id": filters.InboxIds,
			"c.org_id":   filters.OrgId,
			"sar.status": "completed",
		}).
		Where(squirrel.And{
			squirrel.GtOrEq{"c.created_at": filters.Start},
			squirrel.Lt{"c.created_at": filters.End},
		})

	if filters.AssignedUserId != nil {
		query = query.Where(squirrel.Eq{"c.assigned_to": *filters.AssignedUserId})
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return analytics.SarCompletedCount{}, err
	}

	var res analytics.SarCompletedCount
	err = exec.QueryRow(ctx, sql, args...).Scan(&res.Count)
	return res, err
}
