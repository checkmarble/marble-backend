package repositories

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

type UserRepository interface {
	CreateUser(tx Transaction, createUser models.CreateUser) (models.UserId, error)
	UserByUid(tx Transaction, userId models.UserId) (models.User, error)
	UsersOfOrganization(tx Transaction, organizationIDFilter string) ([]models.User, error)
	AllUsers(tx Transaction) ([]models.User, error)
	UserByFirebaseUid(tx Transaction, firebaseUid string) (*models.User, error)
	UserByEmail(tx Transaction, email string) (*models.User, error)
	UpdateFirebaseId(tx Transaction, userId models.UserId, firebaseUid string) error
}

type UserRepositoryPostgresql struct {
	transactionFactory TransactionFactory
	queryBuilder       squirrel.StatementBuilderType
}

func (repo *UserRepositoryPostgresql) CreateUser(tx Transaction, createUser models.CreateUser) (models.UserId, error) {

	userId := models.UserId(uuid.NewString())

	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	var orgId *string
	if len(createUser.OrganizationId) != 0 {
		orgId = &createUser.OrganizationId
	}

	return userId, SqlInsert(
		pgTx,
		repo.queryBuilder.Insert(dbmodels.TABLE_USERS).
			Columns(
				"id",
				"email",
				"firebase_uid",
				"role",
				"organization_id",
			).
			Values(
				string(userId),
				createUser.Email,
				"",
				int(createUser.Role),
				orgId,
			),
	)
}

func (repo *UserRepositoryPostgresql) UserByUid(tx Transaction, userId models.UserId) (models.User, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	return SqlToModel(
		pgTx,
		repo.queryBuilder.
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			Where("id = ?", string(userId)).
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}

func (repo *UserRepositoryPostgresql) UsersOfOrganization(tx Transaction, organizationIDFilter string) ([]models.User, error) {

	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		repo.queryBuilder.
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			Where("organization_id = ?", organizationIDFilter).
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}

func (repo *UserRepositoryPostgresql) AllUsers(tx Transaction) ([]models.User, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		repo.queryBuilder.
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
		repo.queryBuilder.
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
		repo.queryBuilder.
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			Where("email = ?", email).
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
}

func (repo *UserRepositoryPostgresql) UpdateFirebaseId(tx Transaction, userId models.UserId, firebaseUid string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlUpdate(pgTx, repo.queryBuilder.
		Update(dbmodels.TABLE_USERS).
		Set("firebase_uid", firebaseUid).
		Where("id = ?", string(userId)),
	)
}
