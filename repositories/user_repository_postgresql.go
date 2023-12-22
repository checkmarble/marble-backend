package repositories

import (
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/google/uuid"
)

type UserRepository interface {
	CreateUser(tx Transaction, createUser models.CreateUser) (models.UserId, error)
	UpdateUser(tx Transaction, updateUser models.UpdateUser) error
	DeleteUser(tx Transaction, userID models.UserId) error
	DeleteUsersOfOrganization(tx Transaction, organizationId string) error
	UserByID(tx Transaction, userId models.UserId) (models.User, error)
	UsersOfOrganization(tx Transaction, organizationIDFilter string) ([]models.User, error)
	AllUsers(tx Transaction) ([]models.User, error)
	UserByFirebaseUid(tx Transaction, firebaseUid string) (*models.User, error)
	UserByEmail(tx Transaction, email string) (*models.User, error)
	UpdateFirebaseId(tx Transaction, userId models.UserId, firebaseUid string) error
	DeletedUserByEmail(tx Transaction, email string) (*models.User, error)
}

type UserRepositoryPostgresql struct {
	transactionFactory TransactionFactoryPosgresql
}

func (repo *UserRepositoryPostgresql) CreateUser(tx Transaction, createUser models.CreateUser) (models.UserId, error) {

	userId := models.UserId(uuid.NewString())

	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	var organizationId *string
	if len(createUser.OrganizationId) != 0 {
		organizationId = &createUser.OrganizationId
	}

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_USERS).
			Columns(
				"id",
				"email",
				"firebase_uid",
				"role",
				"organization_id",
				"first_name",
				"last_name",
			).
			Values(
				string(userId),
				createUser.Email,
				"",
				int(createUser.Role),
				organizationId,
				createUser.FirstName,
				createUser.LastName,
			),
	)
	return userId, err
}

func (repo *UserRepositoryPostgresql) UpdateUser(tx Transaction, updateUser models.UpdateUser) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

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
	if updateUser.Reactivated {
		query = query.Set("deleted_at", nil)
	}

	_, err := pgTx.ExecBuilder(query)
	return err
}

func (repo *UserRepositoryPostgresql) DeleteUser(tx Transaction, userID models.UserId) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().
			Update(dbmodels.TABLE_USERS).
			Where(squirrel.Eq{"id": userID}).
			Set("deleted_at", squirrel.Expr("NOW()")),
	)
	return err
}

func (repo *UserRepositoryPostgresql) DeleteUsersOfOrganization(tx Transaction, organizationId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Delete(dbmodels.TABLE_USERS).Where("organization_id = ?", string(organizationId)),
	)
	return err
}

func (repo *UserRepositoryPostgresql) UserByID(tx Transaction, userId models.UserId) (models.User, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
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

func (repo *UserRepositoryPostgresql) UsersOfOrganization(tx Transaction, organizationIDFilter string) ([]models.User, error) {

	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
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

func (repo *UserRepositoryPostgresql) AllUsers(tx Transaction) ([]models.User, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}

func (repo *UserRepositoryPostgresql) UserByFirebaseUid(tx Transaction, firebaseUid string) (*models.User, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToOptionalModel(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			Where("firebase_uid = ?", firebaseUid).
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}

func (repo *UserRepositoryPostgresql) UserByEmail(tx Transaction, email string) (*models.User, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToOptionalModel(
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

func (repo *UserRepositoryPostgresql) DeletedUserByEmail(tx Transaction, email string) (*models.User, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToOptionalModel(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			Where("email = ?", email).
			Where("deleted_at IS NOT NULL").
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}

func (repo *UserRepositoryPostgresql) UpdateFirebaseId(tx Transaction, userId models.UserId, firebaseUid string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(NewQueryBuilder().
		Update(dbmodels.TABLE_USERS).
		Set("firebase_uid", firebaseUid).
		Where("id = ?", string(userId)),
	)
	return err
}
