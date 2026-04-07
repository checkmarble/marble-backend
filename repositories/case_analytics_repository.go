package repositories

import (
	"context"
	"fmt"
	"time"

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
			"sar.status":     "completed",
			"sar.deleted_at": nil,
		}).
		Where(squirrel.And{
			squirrel.GtOrEq{"sar.completed_at": filters.Start},
			squirrel.Lt{"sar.completed_at": filters.End},
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

func (repo MarbleDbRepository) OpenCasesByAge(
	ctx context.Context,
	exec Executor,
	filters analytics.CaseAnalyticsFilter,
) ([]analytics.OpenCasesByAge, error) {
	query := NewQueryBuilder().
		Select(
			`case
				when now() - created_at < interval '3 days' then '0-2'
				when now() - created_at < interval '10 days' then '3-10'
				when now() - created_at < interval '30 days' then '11-30'
				else '31+'
			end as bracket`,
			"count(*) as count",
		).
		From(dbmodels.TABLE_CASES).
		Where(squirrel.Eq{
			"inbox_id": filters.InboxIds,
			"org_id":   filters.OrgId,
		}).
		Where(squirrel.NotEq{"status": "closed"}).
		GroupBy("1")

	if filters.AssignedUserId != nil {
		query = query.Where(squirrel.Eq{"assigned_to": *filters.AssignedUserId})
	}

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (analytics.OpenCasesByAge, error) {
		var res analytics.OpenCasesByAge
		err := row.Scan(&res.Bracket, &res.Count)
		return res, err
	})
}

func (repo MarbleDbRepository) SarDelayByTimeStats(
	ctx context.Context,
	exec Executor,
	filters analytics.CaseAnalyticsFilter,
) ([]analytics.SarDelay, error) {
	query := NewQueryBuilder().
		Select(
			fmt.Sprintf("(sar.completed_at + interval '%d s')::date as date", filters.TzOffsetSeconds),
			"sum(extract(epoch from (sar.completed_at - c.created_at)) / 86400) as sum_days",
			"max(extract(epoch from (sar.completed_at - c.created_at)) / 86400) as max_days",
			"count(*) as count_sars",
		).
		From(dbmodels.TABLE_SUSPICIOUS_ACTIVITY_REPORTS + " sar").
		Join(dbmodels.TABLE_CASES + " c on c.id = sar.case_id").
		Where(squirrel.Eq{
			"c.inbox_id": filters.InboxIds,
			"c.org_id":   filters.OrgId,
			"sar.status":     "completed",
			"sar.deleted_at": nil,
		}).
		Where(squirrel.And{
			squirrel.GtOrEq{"sar.completed_at": filters.Start},
			squirrel.Lt{"sar.completed_at": filters.End},
		}).
		GroupBy("date").
		OrderBy("date")

	if filters.AssignedUserId != nil {
		query = query.Where(squirrel.Eq{"c.assigned_to": *filters.AssignedUserId})
	}

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (analytics.SarDelay, error) {
		var res analytics.SarDelay
		err := row.Scan(&res.Date, &res.SumDays, &res.MaxDays, &res.CountSars)
		return res, err
	})
}

func (repo MarbleDbRepository) SarDelayDistribution(
	ctx context.Context,
	exec Executor,
	filters analytics.CaseAnalyticsFilter,
) ([]analytics.SarDelayDistribution, error) {
	query := NewQueryBuilder().
		Select(
			`case
				when extract(epoch from (sar.completed_at - c.created_at)) / 86400 < 3 then '0-2'
				when extract(epoch from (sar.completed_at - c.created_at)) / 86400 < 10 then '3-10'
				when extract(epoch from (sar.completed_at - c.created_at)) / 86400 < 30 then '11-30'
				else '31+'
			end as bracket`,
			"count(*) as count",
		).
		From(dbmodels.TABLE_SUSPICIOUS_ACTIVITY_REPORTS + " sar").
		Join(dbmodels.TABLE_CASES + " c on c.id = sar.case_id").
		Where(squirrel.Eq{
			"c.inbox_id":     filters.InboxIds,
			"c.org_id":       filters.OrgId,
			"sar.status":     "completed",
			"sar.deleted_at": nil,
		}).
		Where(squirrel.And{
			squirrel.GtOrEq{"sar.completed_at": filters.Start},
			squirrel.Lt{"sar.completed_at": filters.End},
		}).
		GroupBy("1")

	if filters.AssignedUserId != nil {
		query = query.Where(squirrel.Eq{"c.assigned_to": *filters.AssignedUserId})
	}

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (
		analytics.SarDelayDistribution, error,
	) {
		var res analytics.SarDelayDistribution
		err := row.Scan(&res.Bracket, &res.Count)
		return res, err
	})
}

func (repo MarbleDbRepository) CaseStatusByDate(
	ctx context.Context,
	exec Executor,
	filters analytics.CaseAnalyticsFilter,
) ([]analytics.CaseStatusByDate, error) {
	cte := WithCtes("case_statuses", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
		q := b.Select(
			fmt.Sprintf("created_at + interval '%d s' as created_at", filters.TzOffsetSeconds),
			"status",
			"snoozed_until is not null and snoozed_until > now() as snoozed",
		).
			From(dbmodels.TABLE_CASES).
			Where(squirrel.Eq{
				"org_id":   filters.OrgId,
				"inbox_id": filters.InboxIds,
			}).
			Where(squirrel.And{
				squirrel.GtOrEq{"created_at": filters.Start},
				squirrel.Lt{"created_at": filters.End},
			})

		if filters.AssignedUserId != nil {
			q = q.Where(squirrel.Eq{"assigned_to": *filters.AssignedUserId})
		}
		return q
	})

	cte = cte.With("data", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
		return b.
			Select(
				"created_at::date as date",
				"count(*) filter (where snoozed) as snoozed",
				"count(*) filter (where not snoozed and status = 'pending') as pending",
				"count(*) filter (where not snoozed and status = 'investigating') as investigating",
				"count(*) filter (where not snoozed and status = 'closed') as closed",
			).
			From("case_statuses").
			GroupBy("created_at::date").
			OrderBy("created_at::date")
	})

	query := squirrel.
		Select(
			"days::date as date",
			"coalesce(data.snoozed, 0) as snoozed",
			"coalesce(data.pending, 0) as pending",
			"coalesce(data.investigating, 0) as investigating",
			"coalesce(data.closed, 0) as closed",
		).
		PrefixExpr(cte).
		From(fmt.Sprintf(
			"generate_series(('%s')::date, ('%s')::date, '1 day') as days",
			filters.Start.Add(time.Duration(filters.TzOffsetSeconds)*time.Second).Format("2006-01-02"),
			filters.End.Add(time.Duration(filters.TzOffsetSeconds)*time.Second).Format("2006-01-02"),
		)).
		LeftJoin("data on data.date = days::date").
		OrderBy("date")

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows[analytics.CaseStatusByDate](rows, pgx.RowToStructByName)
}

func (repo MarbleDbRepository) CaseStatusByInbox(
	ctx context.Context,
	exec Executor,
	filters analytics.CaseAnalyticsFilter,
) ([]analytics.CaseStatusByInbox, error) {
	sql := NewQueryBuilder().
		Select(
			"i.name as inbox",
			"count(*) filter (where snoozed_until is not null and snoozed_until > now()) as snoozed",
			"count(*) filter (where not (snoozed_until is not null and snoozed_until > now()) and c.status = 'pending') as pending",
			"count(*) filter (where not (snoozed_until is not null and snoozed_until > now()) and c.status = 'investigating') as investigating",
			"count(*) filter (where not (snoozed_until is not null and snoozed_until > now()) and c.status = 'closed') as closed",
		).
		From(dbmodels.TABLE_CASES+" c").
		Join("inboxes i on i.id = c.inbox_id").
		Where(squirrel.Eq{
			"c.org_id":   filters.OrgId,
			"c.inbox_id": filters.InboxIds,
		}).
		Where(squirrel.And{
			squirrel.GtOrEq{"c.created_at": filters.Start},
			squirrel.Lt{"c.created_at": filters.End},
		}).
		GroupBy("i.name").
		OrderBy("count(*) desc", "i.name")

	if filters.AssignedUserId != nil {
		sql = sql.Where(squirrel.Eq{"c.assigned_to": *filters.AssignedUserId})
	}

	return SqlToListOfRow(ctx, exec, sql, func(row pgx.CollectableRow) (analytics.CaseStatusByInbox, error) {
		var res analytics.CaseStatusByInbox
		err := row.Scan(&res.Inbox, &res.Snoozed, &res.Pending, &res.Investigating, &res.Closed)
		return res, err
	})
}
