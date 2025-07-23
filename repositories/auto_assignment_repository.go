package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repo *MarbleDbRepository) FindAutoAssignableUsers(ctx context.Context, exec Executor, orgId string, limit int) ([]models.UserWithCaseCount, error) {
	sql := `
		select u.*, count(distinct c.id) filter (where c.status != 'closed') as case_count
		from inbox_users iu
		inner join users u on
		  u.organization_id = $1 and
		  u.id = iu.user_id and
		  u.deleted_at is null and
		  iu.auto_assignable
		left join cases c on c.assigned_to = u.id
		where
		  not exists (
		    select 1
		    from user_unavailabilities uu
		    where
				uu.org_id = $2 and
				uu.user_id = iu.user_id and
				now() between uu.from_date and uu.until_date
	  	  )
		group by u.id
		having count(distinct c.id) filter (where c.status != 'closed') < $3
		order by case_count
	`

	rows, err := exec.Query(ctx, sql, orgId, orgId, limit)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	dbUsers, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbmodels.DbAssignableUserWithCaseCount])
	if err != nil {
		return nil, err
	}

	users, err := pure_utils.MapErr(dbUsers, dbmodels.AdaptAssignableUserWithCaseCount)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (repo *MarbleDbRepository) FindNextAutoAssignableUserForInbox(ctx context.Context, exec Executor, orgId string, inboxId uuid.UUID, limit int) (*models.User, error) {
	sql := `
		select u.*, count(distinct c.id) filter (where c.status != 'closed') as case_count
		from inbox_users iu
		inner join users u on
		  u.organization_id = $1 and
		  u.id = iu.user_id and
		  u.deleted_at is null and
		  iu.inbox_id = $2 and
		  iu.auto_assignable
		left join cases c on c.assigned_to = u.id
		where
		  not exists (
		    select 1
		    from user_unavailabilities uu
		    where
				uu.org_id = $3 and
				uu.user_id = iu.user_id and
				now() between uu.from_date and uu.until_date
	  	  )
		group by u.id
		having count(distinct c.id) filter (where c.status != 'closed') < $4
		order by case_count, id asc
		limit 1
	`

	rows, err := exec.Query(ctx, sql, orgId, inboxId, orgId, limit)
	if err != nil {
		return nil, err
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbmodels.DbAssignableUserWithCaseCount])
	if err != nil {
		return nil, err
	}

	user, err := dbmodels.AdaptAssignableUserWithCaseCount(row)
	if err != nil {
		return nil, err
	}

	return &user.User, nil
}

func (repo *MarbleDbRepository) FindAutoAssignableCases(ctx context.Context, exec Executor, limit int) ([]models.Case, error) {
	sql := NewQueryBuilder().
		Select(columnsNames("c", dbmodels.SelectCaseColumn)...).
		From(dbmodels.TABLE_CASES+" c").
		InnerJoin(dbmodels.TABLE_INBOXES+" i on i.id = c.inbox_id").
		Where(squirrel.Eq{
			"i.auto_assign_enabled": true,
			"c.assigned_to":         nil,
			"c.snoozed_until":       nil,
		}).
		Where("c.status != ?", models.CaseClosed).
		OrderBy("c.created_at").
		Limit(uint64(limit))

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptCase)
}
