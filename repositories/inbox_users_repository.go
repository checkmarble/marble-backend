package repositories

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

func selectInboxUsers() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(columnsNames("u", dbmodels.SelectInboxUserWithOrgIdColumn)...).
		Column("i.organization_id").
		From(dbmodels.TABLE_INBOX_USERS + " AS u").
		Join(dbmodels.TABLE_INBOXES + " AS i ON i.id = inbox_id")
}

func (repo *MarbleDbRepository) GetInboxUserById(tx Transaction, inboxUserId string) (models.InboxUser, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(pgTx,
		selectInboxUsers().
			Where(squirrel.Eq{"u.id": inboxUserId}),
		dbmodels.AdaptInboxUserWithOrgId,
	)
}

func (repo *MarbleDbRepository) ListInboxUsers(tx Transaction, filters models.InboxUserFilterInput) ([]models.InboxUser, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := selectInboxUsers()

	if filters.InboxId != "" {
		query = query.Where(squirrel.Eq{"u.inbox_id": filters.InboxId})
	}
	if filters.UserId != "" {
		query = query.Where(squirrel.Eq{"u.user_id": filters.UserId})
	}

	return SqlToListOfModels(pgTx,
		query,
		dbmodels.AdaptInboxUserWithOrgId,
	)
}

func (repo *MarbleDbRepository) CreateInboxUser(tx Transaction, input models.CreateInboxUserInput, newInboxUserId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_INBOX_USERS).
			Columns(
				"id",
				"inbox_id",
				"user_id",
				"role",
			).
			Values(
				newInboxUserId,
				input.InboxId,
				input.UserId,
				input.Role,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateInboxUser(tx Transaction, inboxUserId string, role models.InboxUserRole) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Update(dbmodels.TABLE_INBOX_USERS).
			Set("role", role).
			Set("updated_at", "NOW()").
			Where(squirrel.Eq{"id": inboxUserId}),
	)
	return err
}

func (repo *MarbleDbRepository) DeleteInboxUser(tx Transaction, inboxUserId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Delete(dbmodels.TABLE_INBOX_USERS).
			Where(squirrel.Eq{"id": inboxUserId}),
	)
	return err
}

// Zoéé update mocks
