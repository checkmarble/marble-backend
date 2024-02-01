package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/google/uuid"
)

type UserRepository interface {
	CreateUser(ctx context.Context, tx Transaction, createUser models.CreateUser) (models.UserId, error)
	UpdateUser(ctx context.Context, tx Transaction, updateUser models.UpdateUser) error
	DeleteUser(ctx context.Context, tx Transaction, userID models.UserId) error
	DeleteUsersOfOrganization(ctx context.Context, tx Transaction, organizationId string) error
	UserByID(ctx context.Context, tx Transaction, userId models.UserId) (models.User, error)
	UsersOfOrganization(ctx context.Context, tx Transaction, organizationIDFilter string) ([]models.User, error)
	AllUsers(ctx context.Context, tx Transaction) ([]models.User, error)
	UserByEmail(ctx context.Context, tx Transaction, email string) (*models.User, error)
}

type UserRepositoryPostgresql struct {
	transactionFactory TransactionFactoryPosgresql
}

func (repo *UserRepositoryPostgresql) CreateUser(ctx context.Context, tx Transaction, createUser models.CreateUser) (models.UserId, error) {

	userId := models.UserId(uuid.NewString())

	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	var organizationId *string
	if len(createUser.OrganizationId) != 0 {
		organizationId = &createUser.OrganizationId
	}

	_, err := pgTx.ExecBuilder(
		ctx,
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

func (repo *UserRepositoryPostgresql) UpdateUser(ctx context.Context, tx Transaction, updateUser models.UpdateUser) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

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

	_, err := pgTx.ExecBuilder(ctx, query)
	return err
}

func (repo *UserRepositoryPostgresql) DeleteUser(ctx context.Context, tx Transaction, userID models.UserId) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().
			Update(dbmodels.TABLE_USERS).
			Where(squirrel.Eq{"id": userID}).
			Set("deleted_at", squirrel.Expr("NOW()")),
	)
	return err
}

func (repo *UserRepositoryPostgresql) DeleteUsersOfOrganization(ctx context.Context, tx Transaction, organizationId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().Delete(dbmodels.TABLE_USERS).Where("organization_id = ?", string(organizationId)),
	)
	return err
}

func (repo *UserRepositoryPostgresql) UserByID(ctx context.Context, tx Transaction, userId models.UserId) (models.User, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToModel(
		ctx,
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			Where(squirrel.Eq{"id": userId}).
			Where("deleted_at IS NULL").
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}

func (repo *UserRepositoryPostgresql) UsersOfOrganization(ctx context.Context, tx Transaction, organizationIDFilter string) ([]models.User, error) {

	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToListOfModels(
		ctx,
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			Where("organization_id = ?", organizationIDFilter).
			Where("deleted_at IS NULL").
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}

func (repo *UserRepositoryPostgresql) AllUsers(ctx context.Context, tx Transaction) ([]models.User, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToListOfModels(
		ctx,
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}

func (repo *UserRepositoryPostgresql) UserByEmail(ctx context.Context, tx Transaction, email string) (*models.User, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToOptionalModel(
		ctx,
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			Where("email = ?", email).
			Where("deleted_at IS NULL").
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}
