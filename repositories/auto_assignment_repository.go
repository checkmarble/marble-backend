package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// This SQL query and the one below basically do the following
//   - For an organization;
//   - Select all active users that:
//   - Configured as part of a rotation
//   - Have less than X active cases assigned to them (through the lateral subquery)
//   - Are currently available (through the `not exists` subquery)
func (repo *MarbleDbRepository) FindAutoAssignableUsers(ctx context.Context, exec Executor, orgId string, limit int) ([]models.UserWithCaseCount, error) {
	sql := `
		select u.*, count(distinct c.id) as case_count
		from inbox_users iu
		inner join users u on
		  u.organization_id = $1 and
		  u.id = iu.user_id and
		  u.deleted_at is null and
		  iu.auto_assignable
		  left join lateral (
		    select c.id
		    from cases c
		    where
		  	  c.org_id = u.organization_id and
		  	  c.assigned_to = u.id and
		  	  c.status != 'closed' and
		  	  coalesce(c.snoozed_until, to_timestamp(0)) < now()
			  limit $2
		  ) c on true
		where
		  not exists (
		    select 1
		    from user_unavailabilities uu
		    where
				uu.org_id = $1 and
				uu.user_id = iu.user_id and
				now() between uu.from_date and uu.until_date
	  	  )
		group by u.id
		having count(distinct c.id) < $2
		order by case_count
	`

	rows, err := exec.Query(ctx, sql, orgId, limit)

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
		select u.*, count(distinct c.id) as case_count
		from inbox_users iu
		inner join users u on
		  u.organization_id = $1 and
		  u.id = iu.user_id and
		  u.deleted_at is null and
		  iu.inbox_id = $2 and
		  iu.auto_assignable
		left join lateral (
		  select c.id
		  from cases c
		  where
			c.org_id = u.organization_id and
			c.assigned_to = u.id and
			c.status != 'closed' and
		    coalesce(c.snoozed_until, to_timestamp(0)) < now()
		  limit $3
		) c on true
		where
		  not exists (
		    select 1
		    from user_unavailabilities uu
		    where
				uu.org_id = $1 and
				uu.user_id = iu.user_id and
				now() between uu.from_date and uu.until_date
	  	  )
		group by u.id
		having count(distinct c.id) < $3
		order by case_count, id asc
		limit 1
	`

	rows, err := exec.Query(ctx, sql, orgId, inboxId, limit)
	if err != nil {
		return nil, err
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbmodels.DbAssignableUserWithCaseCount])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	user, err := dbmodels.AdaptAssignableUserWithCaseCount(row)
	if err != nil {
		return nil, err
	}

	return &user.User, nil
}

func (repo *MarbleDbRepository) FindAutoAssignableCases(ctx context.Context, exec Executor, orgId string, limit int) ([]models.Case, error) {
	sql := `
		with assignable_inboxes as (
		  select distinct iu.inbox_id
		  from inbox_users iu
		  inner join users u on
		    u.organization_id = $1 and
		    u.id = iu.user_id and
		    u.deleted_at is null and
		    iu.auto_assignable
		    left join lateral (
		      select c.id
		      from cases c
		      where
		        c.org_id = u.organization_id and
		        c.assigned_to = u.id and
		        c.status != 'closed' and
		        coalesce(c.snoozed_until, to_timestamp(0)) < now()
		      limit $2
		    ) c on true
		  where
		    not exists (
		      select 1
		      from user_unavailabilities uu
		      where
		      uu.org_id = $1 and
		      uu.user_id = iu.user_id and
		      now() between uu.from_date and uu.until_date
		      )
		  group by iu.inbox_id
		  having count(distinct c.id) < $2
		)
		select c.*
		from assignable_inboxes ai
		inner join inboxes i on i.id = ai.inbox_id and i.auto_assign_enabled
		inner join lateral(
		  select c.*
		  from cases c
		  where
		    c.inbox_id = i.id and
		    c.org_id = $1 and
		    c.status != 'closed' and
		    c.assigned_to is null
		  limit $2
		) c on true
	`

	rows, err := exec.Query(ctx, sql, orgId, limit)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	dbCases, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbmodels.DBCase])
	if err != nil {
		return nil, err
	}

	cases, err := pure_utils.MapErr(dbCases, dbmodels.AdaptCase)
	if err != nil {
		return nil, err
	}

	return cases, nil
}
