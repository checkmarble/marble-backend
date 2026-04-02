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

func (repo MarbleDbRepository) CasesCreatedByTimeStats(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	inboxIds []uuid.UUID,
	assignedUserId *string,
	start time.Time,
	end time.Time,
	tzOffsetSeconds int,
) ([]analytics.CasesCreated, error) {
	query := NewQueryBuilder().
		Select(
			fmt.Sprintf("(created_at + interval '%d s')::date as date", tzOffsetSeconds),
			"count(*) as count",
		).
		From(dbmodels.TABLE_CASES).
		Where(squirrel.Eq{
			"inbox_id": inboxIds,
			"org_id":   orgId,
		}).
		Where(squirrel.And{
			squirrel.GtOrEq{"created_at": start},
			squirrel.Lt{"created_at": end},
		}).
		GroupBy("date").
		OrderBy("date")

	if assignedUserId != nil {
		query = query.Where(squirrel.Eq{"assigned_to": *assignedUserId})
	}

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (analytics.CasesCreated, error) {
		var res analytics.CasesCreated
		err := row.Scan(&res.Date, &res.Count)
		return res, err
	})
}
