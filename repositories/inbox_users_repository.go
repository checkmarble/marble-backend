package repositories

import (
	"context"

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

func (repo *MarbleDbRepository) GetInboxUserById(ctx context.Context, tx Transaction, inboxUserId string) (models.InboxUser, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToModel(
		ctx,
		pgTx,
		selectInboxUsers().
			Where(squirrel.Eq{"u.id": inboxUserId}),
		dbmodels.AdaptInboxUserWithOrgId,
	)
}

func (repo *MarbleDbRepository) ListInboxUsers(ctx context.Context, tx Transaction, filters models.InboxUserFilterInput) ([]models.InboxUser, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	query := selectInboxUsers()

	if filters.InboxId != "" {
		query = query.Where(squirrel.Eq{"u.inbox_id": filters.InboxId})
	}
	if filters.UserId != "" {
		query = query.Where(squirrel.Eq{"u.user_id": filters.UserId})
	}

	return SqlToListOfModels(
		ctx,
		pgTx,
		query,
		dbmodels.AdaptInboxUserWithOrgId,
	)
}

func (repo *MarbleDbRepository) CreateInboxUser(ctx context.Context, tx Transaction, input models.CreateInboxUserInput, newInboxUserId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
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

func (repo *MarbleDbRepository) UpdateInboxUser(ctx context.Context, tx Transaction, inboxUserId string, role models.InboxUserRole) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().Update(dbmodels.TABLE_INBOX_USERS).
			Set("role", role).
			Set("updated_at", "NOW()").
			Where(squirrel.Eq{"id": inboxUserId}),
	)
	return err
}

func (repo *MarbleDbRepository) DeleteInboxUser(ctx context.Context, tx Transaction, inboxUserId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().Delete(dbmodels.TABLE_INBOX_USERS).
			Where(squirrel.Eq{"id": inboxUserId}),
	)
	return err
}
