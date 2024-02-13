package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

func (repo *MarbleDbRepository) GetInboxById(ctx context.Context, tx Transaction_deprec, inboxId string) (models.Inbox, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToModel(
		ctx,
		pgTx,
		selectInboxesJoinUsers().Where(squirrel.Eq{"i.id": inboxId}),
		dbmodels.AdaptInboxWithUsers,
	)
}

func (repo *MarbleDbRepository) ListInboxes(ctx context.Context, tx Transaction_deprec, organizationId string, inboxIds []string, withCaseCount bool) ([]models.Inbox, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	query := selectInboxesJoinUsers().
		Where(squirrel.Eq{"i.status": models.InboxStatusActive}).
		Where(squirrel.Eq{"i.organization_id": organizationId})

	if len(inboxIds) > 0 {
		query = query.Where(squirrel.Eq{"i.id": inboxIds})
	}

	if withCaseCount {
		query = query.Column("(SELECT count(distinct c.id) FROM " + dbmodels.TABLE_CASES + " AS c WHERE c.inbox_id = i.id) AS cases_count")
		return SqlToListOfModels(ctx, pgTx, query, dbmodels.AdaptInboxWithCasesCount)
	}

	return SqlToListOfModels(
		ctx,
		pgTx,
		query,
		dbmodels.AdaptInboxWithUsers,
	)
}

func selectInboxesJoinUsers() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(columnsNames("i", dbmodels.SelectInboxColumn)...).
		Column(
			fmt.Sprintf(
				"array_agg(row(%s) ORDER BY u.created_at) FILTER (WHERE u.id IS NOT NULL) as inbox_users",
				strings.Join(columnsNames("u", dbmodels.SelectInboxUserColumn), ","),
			),
		).
		From(dbmodels.TABLE_INBOXES + " AS i").
		LeftJoin(dbmodels.TABLE_INBOX_USERS + " AS u ON u.inbox_id = i.id").
		GroupBy("i.id").
		OrderBy("i.created_at DESC")
}

func (repo *MarbleDbRepository) CreateInbox(ctx context.Context, tx Transaction_deprec, input models.CreateInboxInput, newInboxId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().Insert(dbmodels.TABLE_INBOXES).
			Columns(
				"id",
				"organization_id",
				"name",
			).
			Values(
				newInboxId,
				input.OrganizationId,
				input.Name,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateInbox(ctx context.Context, tx Transaction_deprec, inboxId, name string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().Update(dbmodels.TABLE_INBOXES).
			Set("name", name).
			Set("updated_at", squirrel.Expr("NOW()")).
			Where(squirrel.Eq{"id": inboxId}),
	)
	return err
}

func (repo *MarbleDbRepository) SoftDeleteInbox(ctx context.Context, tx Transaction_deprec, inboxId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().Update(dbmodels.TABLE_INBOXES).
			Set("status", models.InboxStatusInactive).
			Set("updated_at", squirrel.Expr("NOW()")).
			Where(squirrel.Eq{"id": inboxId}),
	)
	return err
}
