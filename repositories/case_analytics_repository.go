package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CaseAnalyticsRepository struct{}

func (repo CaseAnalyticsRepository) CasesCreated(ctx context.Context, exec Executor,
	orgId uuid.UUID, inboxIds []uuid.UUID, assignedUserId *string,
	start, end time.Time, tzOffsetSeconds int,
) ([]analytics.CasesCreated, error) {
	query := squirrel.
		Select(
			fmt.Sprintf("(created_at + interval '%d s')::date as date", tzOffsetSeconds),
			"count(*) as count",
		).
		From(dbmodels.TABLE_CASES).
		Where("org_id = ?", orgId).
		Where(squirrel.Eq{"inbox_id": inboxIds}).
		Where("created_at >= ? and created_at < ?", start, end).
		GroupBy("date").
		OrderBy("date")

	if assignedUserId != nil {
		query = query.Where("assigned_to = ?", *assignedUserId)
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows[analytics.CasesCreated](rows, pgx.RowToStructByName)
}
