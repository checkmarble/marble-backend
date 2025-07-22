package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/jackc/pgx/v5"
)

func (repo *MarbleDbRepository) FindAutoAssignableUsers(ctx context.Context, exec Executor, inboxId string) ([]models.UserWithCaseCount, error) {
	sql := `
		select u.*, count(*) filter (where c.id is not null) as case_count
		from inbox_users iu
		inner join users u on u.id = iu.user_id
		left join cases c on c.assigned_to = u.id
		where
		  iu.inbox_id = $1 and
		  auto_assignable = true and
		  not exists (
		    select 1
		    from user_unavailabilities uu
		    where
				uu.org_id = $2 and
				uu.user_id = iu.user_id and
				now() between uu.from_date and uu.until_date
	  	  )
		group by u.id
		having count(*) filter (where c.id is not null) < $3
		order by case_count
	`

	rows, err := exec.Query(ctx, sql, inboxId, orgId, limit)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	users := make([]models.UserWithCaseCount, 0)

	for rows.Next() {
		dbUser, err := pgx.RowToStructByName[dbmodels.DbAssignableUserWithCaseCount](rows)
		if err != nil {
			return nil, err
		}

		user, err := dbmodels.AdaptAssignableUserWithCaseCount(dbUser)
		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	return users, nil
}

func (repo *MarbleDbRepository) FindAssignableCases(ctx context.Context, exec Executor, inboxId string, limit int) ([]models.Case, error) {
	sql := NewQueryBuilder().
		Select(dbmodels.SelectCaseColumn...).
		From(dbmodels.TABLE_CASES).
		Where(squirrel.Eq{
			"inbox_id":      inboxId,
			"assigned_to":   nil,
			"snoozed_until": nil,
		}).
		Where("status != ?", models.CaseClosed).
		OrderBy("created_at").
		Limit(uint64(limit))

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptCase)
}
