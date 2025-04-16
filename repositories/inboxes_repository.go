package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

func (repo *MarbleDbRepository) GetInboxById(ctx context.Context, exec Executor, inboxId string) (models.Inbox, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Inbox{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectInboxesJoinUsers().Where(squirrel.Eq{"i.id": inboxId}),
		dbmodels.AdaptInboxWithUsers,
	)
}

func (repo *MarbleDbRepository) ListInboxes(ctx context.Context, exec Executor,
	organizationId string, inboxIds []string, withCaseCount bool,
) ([]models.Inbox, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectInboxesJoinUsers().
		Where(squirrel.Eq{"i.status": models.InboxStatusActive}).
		Where(squirrel.Eq{"i.organization_id": organizationId})

	if len(inboxIds) > 0 {
		query = query.Where(squirrel.Eq{"i.id": inboxIds})
	}

	if withCaseCount {
		query = query.Column("(SELECT count(distinct c.id) FROM " + dbmodels.TABLE_CASES +
			" AS c WHERE c.inbox_id = i.id) AS cases_count")
		return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptInboxWithCasesCount)
	}

	return SqlToListOfModels(
		ctx,
		exec,
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

func (repo *MarbleDbRepository) CreateInbox(ctx context.Context, exec Executor, input models.CreateInboxInput, newInboxId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_INBOXES).
			Columns(
				"id",
				"organization_id",
				"name",
				"escalation_inbox_id",
			).
			Values(
				newInboxId,
				input.OrganizationId,
				input.Name,
				input.EscalationInboxId,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateInbox(ctx context.Context, exec Executor, inboxId, name string, escalationInboxId *string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Update(dbmodels.TABLE_INBOXES).
			Set("name", name).
			Set("updated_at", squirrel.Expr("NOW()")).
			Set("escalation_inbox_id", escalationInboxId).
			Where(squirrel.Eq{"id": inboxId}),
	)
	return err
}

func (repo *MarbleDbRepository) SoftDeleteInbox(ctx context.Context, exec Executor, inboxId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Update(dbmodels.TABLE_INBOXES).
			Set("status", models.InboxStatusInactive).
			Set("updated_at", squirrel.Expr("NOW()")).
			Where(squirrel.Eq{"id": inboxId}),
	)
	return err
}
