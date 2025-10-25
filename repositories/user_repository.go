package repositories

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/redis/go-redis/v9"

	"github.com/google/uuid"
)

type UserRepository interface {
	CreateUser(ctx context.Context, exec Executor, createUser models.CreateUser) (string, error)
	UpdateUser(ctx context.Context, exec Executor, updateUser models.UpdateUser) error
	DeleteUser(ctx context.Context, exec Executor, userID models.UserId) error
	DeleteUsersOfOrganization(ctx context.Context, exec Executor, organizationId uuid.UUID) error
	UserById(ctx context.Context, exec Executor, userId string) (models.User, error)
	ListUsers(ctx context.Context, exec Executor, organizationId *uuid.UUID) ([]models.User, error)
	UserByEmail(ctx context.Context, exec Executor, email string) (*models.User, error)
	HasUsers(ctx context.Context, exec Executor) (bool, error)
}

func (repo *MarbleDbRepository) CreateUser(ctx context.Context, exec Executor, createUser models.CreateUser) (string, error) {
	userId := uuid.NewString()

	if err := validateMarbleDbExecutor(exec); err != nil {
		return "", err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_USERS).
			Columns(
				"id",
				"email",
				"role",
				"organization_id",
				"partner_id",
				"first_name",
				"last_name",
			).
			Values(
				userId,
				createUser.Email,
				int(createUser.Role),
				createUser.OrganizationId,
				createUser.PartnerId,
				createUser.FirstName,
				createUser.LastName,
			),
	)
	return userId, err
}

func (repo *MarbleDbRepository) UpdateUser(ctx context.Context, exec Executor, updateUser models.UpdateUser) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().Update(dbmodels.TABLE_USERS).Where(squirrel.Eq{"id": updateUser.UserId})

	if updateUser.Email != nil {
		query = query.Set("email", *updateUser.Email)
	}
	if updateUser.Role != nil && *updateUser.Role != models.NO_ROLE {
		query = query.Set("role", int(*updateUser.Role))
	}
	if updateUser.FirstName != nil {
		query = query.Set("first_name", *updateUser.FirstName)
	}
	if updateUser.LastName != nil {
		query = query.Set("last_name", *updateUser.LastName)
	}

	if err := ExecBuilder(ctx, exec, query); err != nil {
		return err
	}

	return exec.Cache(ctx).Exec(func(c *redis.Client) error {
		return c.Del(ctx, exec.Cache(ctx).Key("user", updateUser.UserId)).Err()
	})
}

func (repo *MarbleDbRepository) DeleteUser(ctx context.Context, exec Executor, userID models.UserId) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Update(dbmodels.TABLE_USERS).
			Where(squirrel.Eq{"id": userID}).
			Set("deleted_at", squirrel.Expr("NOW()")),
	)
	if err != nil {
		return err
	}

	return exec.Cache(ctx).Exec(func(c *redis.Client) error {
		return c.Del(ctx, exec.Cache(ctx).Key("user", string(userID))).Err()
	})
}

func (repo *MarbleDbRepository) DeleteUsersOfOrganization(ctx context.Context, exec Executor, organizationId uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Delete(dbmodels.TABLE_USERS).Where("organization_id = ?", organizationId),
	)
	return err
}

func (repo *MarbleDbRepository) UserById(ctx context.Context, exec Executor, userId string) (models.User, error) {
	if user, err := RedisLoadModel[models.User](ctx, exec.Cache(ctx), exec.Cache(ctx).Key("user", userId)); err == nil {
		return user, nil
	}

	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.User{}, err
	}

	user, err := SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			Where(squirrel.Eq{"id": userId}).
			Where("deleted_at IS NULL").
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
	if err != nil {
		return user, err
	}

	_ = exec.Cache(ctx).SaveModel(ctx, exec.Cache(ctx).Key("user", userId), user, time.Hour)

	return user, nil
}

func (repo *MarbleDbRepository) ListUsers(ctx context.Context, exec Executor, orgId *uuid.UUID) ([]models.User, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.UserFields...).
		From(dbmodels.TABLE_USERS).
		Where("deleted_at IS NULL").
		OrderBy("id")

	if orgId != nil {
		query = query.Where(squirrel.Eq{"organization_id": *orgId})
	}

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptUser,
	)
}

func (repo *MarbleDbRepository) UserByEmail(ctx context.Context, exec Executor, email string) (*models.User, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToOptionalModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			Where("email = ?", email).
			Where("deleted_at IS NULL").
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}

func (repo *MarbleDbRepository) HasUsers(ctx context.Context, exec Executor) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}
	var exists bool
	err := exec.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+dbmodels.TABLE_USERS+" LIMIT 1)").Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
