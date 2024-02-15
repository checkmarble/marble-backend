package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/google/uuid"
)

type UserRepository interface {
	CreateUser(ctx context.Context, exec Executor, createUser models.CreateUser) (models.UserId, error)
	UpdateUser(ctx context.Context, exec Executor, updateUser models.UpdateUser) error
	DeleteUser(ctx context.Context, exec Executor, userID models.UserId) error
	DeleteUsersOfOrganization(ctx context.Context, exec Executor, organizationId string) error
	UserByID(ctx context.Context, exec Executor, userId models.UserId) (models.User, error)
	UsersOfOrganization(ctx context.Context, exec Executor, organizationIDFilter string) ([]models.User, error)
	AllUsers(ctx context.Context, exec Executor) ([]models.User, error)
	UserByEmail(ctx context.Context, exec Executor, email string) (*models.User, error)
}

type UserRepositoryPostgresql struct{}

func (repo *UserRepositoryPostgresql) CreateUser(ctx context.Context, exec Executor, createUser models.CreateUser) (models.UserId, error) {
	userId := models.UserId(uuid.NewString())

	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.UserId(""), err
	}

	var organizationId *string
	if len(createUser.OrganizationId) != 0 {
		organizationId = &createUser.OrganizationId
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
				"first_name",
				"last_name",
			).
			Values(
				string(userId),
				createUser.Email,
				int(createUser.Role),
				organizationId,
				createUser.FirstName,
				createUser.LastName,
			),
	)
	return userId, err
}

func (repo *UserRepositoryPostgresql) UpdateUser(ctx context.Context, exec Executor, updateUser models.UpdateUser) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().Update(dbmodels.TABLE_USERS).Where(squirrel.Eq{"id": updateUser.UserId})

	if updateUser.Email != "" {
		query = query.Set("email", updateUser.Email)
	}
	if updateUser.Role != models.Role(0) {
		query = query.Set("role", int(updateUser.Role))
	}
	if updateUser.FirstName != "" {
		query = query.Set("first_name", updateUser.FirstName)
	}
	if updateUser.LastName != "" {
		query = query.Set("last_name", updateUser.LastName)
	}

	err := ExecBuilder(ctx, exec, query)
	return err
}

func (repo *UserRepositoryPostgresql) DeleteUser(ctx context.Context, exec Executor, userID models.UserId) error {
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
	return err
}

func (repo *UserRepositoryPostgresql) DeleteUsersOfOrganization(ctx context.Context, exec Executor, organizationId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Delete(dbmodels.TABLE_USERS).Where("organization_id = ?", string(organizationId)),
	)
	return err
}

func (repo *UserRepositoryPostgresql) UserByID(ctx context.Context, exec Executor, userId models.UserId) (models.User, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.User{}, err
	}

	return SqlToModel(
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
}

func (repo *UserRepositoryPostgresql) UsersOfOrganization(ctx context.Context, exec Executor, organizationIDFilter string) ([]models.User, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			Where("organization_id = ?", organizationIDFilter).
			Where("deleted_at IS NULL").
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}

func (repo *UserRepositoryPostgresql) AllUsers(ctx context.Context, exec Executor) ([]models.User, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}

func (repo *UserRepositoryPostgresql) UserByEmail(ctx context.Context, exec Executor, email string) (*models.User, error) {
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
