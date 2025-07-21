package repositories

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) GetCurrentUnavailability(ctx context.Context, exec Executor, userId string) (*models.UserUnavailability, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.ColumnsSelectUserUnavailabilities...).
		From(dbmodels.TABLE_USER_UNAVAILABILITIES).
		Where(squirrel.Eq{"user_id": userId}).
		Where(squirrel.Gt{"until_date": time.Now()})

	return SqlToOptionalModel(ctx, exec, sql, dbmodels.AdaptUserUnavailability)
}

func (repo *MarbleDbRepository) InsertUnavailability(ctx context.Context, exec Executor, orgId, userId string, until time.Time) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_USER_UNAVAILABILITIES).
		Columns("org_id", "user_id", "from_date", "until_date").
		Values(orgId, userId, time.Now(), until)

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) UpdateUnavailability(ctx context.Context, exec Executor, id uuid.UUID, until time.Time) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_USER_UNAVAILABILITIES).
		Set("until_date", until).
		Set("updated_at", time.Now()).
		Where("id = ?", id)

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) DeleteUnavailability(ctx context.Context, exec Executor, userId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_USER_UNAVAILABILITIES).
		Set("until_date", time.Now()).
		Set("updated_at", time.Now()).
		Where("user_id = ?", userId).
		Where("until_date > ?", time.Now())

	return ExecBuilder(ctx, exec, sql)
}
